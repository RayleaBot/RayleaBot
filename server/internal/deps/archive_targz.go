package deps

import depsarchive "github.com/RayleaBot/RayleaBot/server/internal/deps/archive"

func extractTarGz(archivePath, destRoot string) error {
	return depsarchive.TarGz(archivePath, destRoot)
}

func extractTarGzWithProgress(archivePath, destRoot string, progress func(extractProgress)) error {
	return depsarchive.TarGzWithProgress(archivePath, destRoot, func(event depsarchive.Progress) {
		if progress == nil {
			return
		}
		progress(extractProgress{
			ExtractedEntries: event.ExtractedEntries,
			TotalEntries:     event.TotalEntries,
			Progress:         event.Progress,
		})
	})
}

func countTarGzEntries(archivePath string) (int, error) {
	return depsarchive.CountTarGzEntries(archivePath)
}
