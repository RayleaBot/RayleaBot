package deps

import depsarchive "github.com/RayleaBot/RayleaBot/server/internal/deps/archive"

func extractZip(archivePath, destRoot string) error {
	return depsarchive.Zip(archivePath, destRoot)
}

func extractZipWithProgress(archivePath, destRoot string, progress func(extractProgress)) error {
	return depsarchive.ZipWithProgress(archivePath, destRoot, func(event depsarchive.Progress) {
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
