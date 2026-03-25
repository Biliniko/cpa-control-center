package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type authImportCandidate struct {
	Path string
	Name string
}

type authImportRunOptions struct {
	RootDirectory    string
	ArchiveDirectory string
	MoveImported     bool
	EmitProgress     bool
}

type uploadedAuthImportCandidate struct {
	candidate   authImportCandidate
	resultIndex int
}

func (b *Backend) ImportAuthFiles(paths []string) (AuthImportResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return AuthImportResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return AuthImportResult{}, err
	}

	candidates := make([]authImportCandidate, 0, len(paths))
	seenPaths := make(map[string]struct{}, len(paths))
	for _, rawPath := range paths {
		cleanPath := strings.TrimSpace(rawPath)
		if cleanPath == "" {
			continue
		}
		normalizedPath := filepath.Clean(cleanPath)
		if _, ok := seenPaths[normalizedPath]; ok {
			continue
		}
		seenPaths[normalizedPath] = struct{}{}
		candidates = append(candidates, authImportCandidate{
			Path: normalizedPath,
			Name: filepath.Base(normalizedPath),
		})
	}
	return b.runManualAuthImport(settings, candidates, authImportRunOptions{
		EmitProgress: true,
	})
}

func (b *Backend) ImportAuthDirectory(directory string) (AuthImportResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return AuthImportResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return AuthImportResult{}, err
	}

	root, err := validateAuthImportDirectory(settings.Locale, directory)
	if err != nil {
		return AuthImportResult{}, err
	}

	settings, err = b.rememberAuthImportSourceDirectory(settings, root)
	if err != nil {
		return AuthImportResult{}, err
	}

	candidates, err := collectAuthImportCandidates(root, settings.AuthImport.ArchiveDirectory, settings.AuthImport.MoveImported)
	if err != nil {
		return AuthImportResult{}, err
	}
	if len(candidates) == 0 {
		return AuthImportResult{}, errors.New(msg(settings.Locale, "error.auth_import_empty_directory"))
	}

	return b.runManualAuthImport(settings, candidates, authImportRunOptions{
		RootDirectory:    root,
		ArchiveDirectory: settings.AuthImport.ArchiveDirectory,
		MoveImported:     settings.AuthImport.MoveImported,
		EmitProgress:     true,
	})
}

func (b *Backend) runScheduledAuthImport(ctx context.Context, settings AppSettings) (AuthImportResult, error) {
	root, err := validateAuthImportDirectory(settings.Locale, settings.AuthImport.SourceDirectory)
	if err != nil {
		return AuthImportResult{}, err
	}

	candidates, err := collectAuthImportCandidates(root, settings.AuthImport.ArchiveDirectory, settings.AuthImport.MoveImported)
	if err != nil {
		return AuthImportResult{}, err
	}

	return b.runAuthImport(ctx, settings, candidates, authImportRunOptions{
		RootDirectory:    root,
		ArchiveDirectory: settings.AuthImport.ArchiveDirectory,
		MoveImported:     settings.AuthImport.MoveImported,
		EmitProgress:     true,
	})
}

func (b *Backend) runManualAuthImport(settings AppSettings, candidates []authImportCandidate, options authImportRunOptions) (AuthImportResult, error) {
	ctx, err := b.beginTask("import", settings.Locale)
	if err != nil {
		return AuthImportResult{}, err
	}
	defer b.endTask()

	result, runErr := b.runAuthImport(ctx, settings, candidates, options)
	status := "success"
	message := authImportSummaryMessage(settings.Locale, result)
	level := "info"
	if runErr != nil {
		status = taskStatus(runErr)
		message = runErr.Error()
		level = "error"
		if status == "cancelled" {
			level = "warning"
		}
	} else if result.SyncError != "" || result.ArchiveFailed > 0 || result.Failed > 0 || result.Skipped > 0 {
		level = "warning"
	}

	b.emitLog("import", level, message)
	b.emitTaskFinished("import", status, message)
	return result, runErr
}

func (b *Backend) runAuthImport(
	ctx context.Context,
	settings AppSettings,
	candidates []authImportCandidate,
	options authImportRunOptions,
) (AuthImportResult, error) {
	result := AuthImportResult{
		Requested: len(candidates),
		Results:   make([]AuthImportFileResult, 0, len(candidates)),
	}
	if options.MoveImported {
		result.ArchiveDirectory = normalizeAuthImportArchiveDirectory(options.ArchiveDirectory)
	}

	if len(candidates) == 0 {
		if options.EmitProgress {
			b.emitProgress("import", "complete", 0, 0, authImportSummaryMessage(settings.Locale, result), true)
		}
		return result, nil
	}

	if options.EmitProgress {
		b.emitProgress("import", "prepare", 0, len(candidates), msg(settings.Locale, "task.import.prepared", len(candidates)), false)
	}

	uploaded := make([]uploadedAuthImportCandidate, 0, len(candidates))
	for index, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		item := AuthImportFileResult{
			Path: candidate.Path,
			Name: candidate.Name,
		}
		switch {
		case strings.TrimSpace(candidate.Name) == "":
			item.Error = msg(settings.Locale, "error.auth_import_invalid_name", candidate.Path)
			result.Failed++
		case !strings.HasSuffix(strings.ToLower(candidate.Name), ".json"):
			item.Error = msg(settings.Locale, "error.auth_import_not_json", candidate.Name)
			result.Skipped++
		default:
			content, err := os.ReadFile(candidate.Path)
			if err != nil {
				item.Error = err.Error()
				result.Failed++
			} else if !json.Valid(content) {
				item.Error = msg(settings.Locale, "error.auth_import_invalid_json", candidate.Name)
				result.Failed++
			} else if err := b.client.UploadAuthFile(ctx, settings, candidate.Name, content); err != nil {
				item.Error = err.Error()
				result.Failed++
			} else {
				item.OK = true
				result.Uploaded++
			}
		}

		result.Results = append(result.Results, item)
		if item.OK {
			uploaded = append(uploaded, uploadedAuthImportCandidate{
				candidate:   candidate,
				resultIndex: len(result.Results) - 1,
			})
		}

		if options.EmitProgress {
			b.emitProgress("import", "upload", index+1, len(candidates), candidate.Name, false)
		}
	}

	if result.Uploaded == 0 {
		if options.EmitProgress {
			b.emitProgress("import", "complete", result.Requested, result.Requested, authImportSummaryMessage(settings.Locale, result), true)
		}
		return result, nil
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}

	if options.EmitProgress {
		b.emitProgress("import", "fetch", 0, 1, msg(settings.Locale, "task.scan.loading_inventory"), false)
	}
	files, err := b.client.FetchAuthFiles(ctx, settings)
	if err != nil {
		result.SyncError = err.Error()
	} else {
		inventory, syncErr := b.syncInventoryFromFilesWithProgress(ctx, settings, files, nil)
		if syncErr != nil {
			result.SyncError = syncErr.Error()
		} else {
			result.Synced = true
			result.Inventory = inventory
		}
	}

	if options.MoveImported && len(uploaded) > 0 {
		archiveDirectory := normalizeAuthImportArchiveDirectory(options.ArchiveDirectory)
		result.ArchiveDirectory = archiveDirectory
		if options.EmitProgress {
			b.emitProgress("import", "archive", 0, len(uploaded), archiveDirectory, false)
		}
		for index, uploadedCandidate := range uploaded {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			if err := archiveImportedCandidate(uploadedCandidate.candidate, archiveDirectory); err != nil {
				result.ArchiveFailed++
				result.Results[uploadedCandidate.resultIndex].ArchiveError = err.Error()
			} else {
				result.Archived++
				result.Results[uploadedCandidate.resultIndex].Archived = true
			}
			if options.EmitProgress {
				b.emitProgress("import", "archive", index+1, len(uploaded), uploadedCandidate.candidate.Name, false)
			}
		}
	}

	if options.EmitProgress {
		b.emitProgress("import", "complete", result.Requested, result.Requested, authImportSummaryMessage(settings.Locale, result), true)
	}
	return result, nil
}

func authImportSummaryMessage(locale string, result AuthImportResult) string {
	if result.Requested == 0 {
		return msg(locale, "task.import.no_files")
	}

	parts := []string{
		msg(locale, "task.import.summary", result.Uploaded, result.Requested, result.Failed, result.Skipped),
	}
	if result.SyncError != "" {
		parts = append(parts, msg(locale, "task.import.sync_failed", result.SyncError))
	}
	if result.Archived > 0 {
		parts = append(parts, msg(locale, "task.import.archived", result.Archived, result.ArchiveDirectory))
	}
	if result.ArchiveFailed > 0 {
		parts = append(parts, msg(locale, "task.import.archive_failed", result.ArchiveFailed))
	}
	return strings.Join(parts, " ")
}

func validateAuthImportDirectory(locale string, directory string) (string, error) {
	root := strings.TrimSpace(directory)
	if root == "" {
		return "", nil
	}
	root = filepath.Clean(root)

	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New(msg(locale, "error.auth_import_directory_invalid"))
	}
	return root, nil
}

func collectAuthImportCandidates(root string, archiveDirectory string, moveImported bool) ([]authImportCandidate, error) {
	candidates := make([]authImportCandidate, 0)
	skipDirectory := ""
	if moveImported {
		skipDirectory = normalizeAuthImportArchiveDirectory(archiveDirectory)
	}

	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if skipDirectory != "" && sameFilePath(current, skipDirectory) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			return nil
		}
		relPath, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		if relPath == "." || strings.HasPrefix(relPath, "..") {
			return nil
		}
		candidates = append(candidates, authImportCandidate{
			Path: current,
			Name: filepath.ToSlash(relPath),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i int, j int) bool {
		return strings.ToLower(candidates[i].Name) < strings.ToLower(candidates[j].Name)
	})
	return candidates, nil
}

func (b *Backend) rememberAuthImportSourceDirectory(settings AppSettings, root string) (AppSettings, error) {
	if root == "" || sameFilePath(settings.AuthImport.SourceDirectory, root) {
		return settings, nil
	}

	settings.AuthImport.SourceDirectory = root
	saved, err := b.store.SaveSettings(settings)
	if err != nil {
		return settings, err
	}
	if b.authImportScheduler != nil {
		b.authImportScheduler.ApplySettings(saved)
	}
	return saved, nil
}

func normalizeAuthImportArchiveDirectory(directory string) string {
	if trimmed := strings.TrimSpace(directory); trimmed != "" {
		return filepath.Clean(trimmed)
	}
	return defaultAuthImportArchiveDirectory()
}

func archiveImportedCandidate(candidate authImportCandidate, archiveDirectory string) error {
	if archiveDirectory == "" {
		return nil
	}

	destinationPath, err := nextAvailableArchivePath(filepath.Join(archiveDirectory, filepath.FromSlash(candidate.Name)))
	if err != nil {
		return err
	}
	if sameFilePath(candidate.Path, destinationPath) {
		return nil
	}
	if err := ensureDir(filepath.Dir(destinationPath)); err != nil {
		return err
	}
	return moveFile(candidate.Path, destinationPath)
}

func nextAvailableArchivePath(destination string) (string, error) {
	if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
		return destination, nil
	} else if err != nil {
		return "", err
	}

	directory := filepath.Dir(destination)
	extension := filepath.Ext(destination)
	baseName := strings.TrimSuffix(filepath.Base(destination), extension)
	for index := 1; ; index++ {
		candidate := filepath.Join(directory, fmt.Sprintf("%s_%d%s", baseName, index, extension))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}

func moveFile(source string, destination string) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	}

	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(destination)
	if err != nil {
		return err
	}

	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		_ = os.Remove(destination)
		return err
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return os.Remove(source)
}

func sameFilePath(left string, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}

	leftPath := filepath.Clean(left)
	rightPath := filepath.Clean(right)
	if absoluteLeft, err := filepath.Abs(leftPath); err == nil {
		leftPath = absoluteLeft
	}
	if absoluteRight, err := filepath.Abs(rightPath); err == nil {
		rightPath = absoluteRight
	}
	return strings.EqualFold(leftPath, rightPath)
}
