package inline

import (
	"github.com/metafates/mangal/downloader"
	"github.com/metafates/mangal/key"
	"github.com/metafates/mangal/log"
	"github.com/metafates/mangal/source"
	"github.com/spf13/viper"
	"os"
)

func Run(options *Options) (err error) {
	if options.Out == nil {
		options.Out = os.Stdout
	}

	var mangas []*source.Manga
	for _, src := range options.Sources {
		m, err := src.Search(options.Query)
		if err != nil {
			return err
		}

		mangas = append(mangas, m...)
	}

	if options.MangaPicker.IsAbsent() && options.ChaptersFilter.IsAbsent() {
		if viper.GetBool(key.MetadataFetchAnilist) {
			for _, manga := range mangas {
				_ = manga.PopulateMetadata(func(string) {})
			}
		}

		marshalled, err := asJson(mangas, options)
		if err != nil {
			return err
		}

		_, err = options.Out.Write(marshalled)
		return err
	}

	// manga picker can only be none if json is set
	if options.MangaPicker.IsAbsent() {
		// preload all chapters
		for _, manga := range mangas {
			if err = prepareManga(manga, options); err != nil {
				return err
			}
		}

		marshalled, err := asJson(mangas, options)
		if err != nil {
			return err
		}

		_, err = options.Out.Write(marshalled)
		return err
	}

	var chapters source.Chapters
	if len(mangas) == 0 {
		if options.Json {
			marshalled, err := asJson([]*source.Manga{}, options)
			if err != nil {
				return err
			}

			_, err = options.Out.Write(marshalled)
			return err
		}

		return nil
	}

	manga := options.MangaPicker.MustGet()(mangas)

	if manga == nil {
		if options.Json {
			marshalled, err := asJson([]*source.Manga{}, options)
			if err != nil {
				return err
			}

			_, err = options.Out.Write(marshalled)
			return err
		}

		return nil
	}

	chapters, err = manga.Source.ChaptersOf(manga)
	if err != nil {
		return err
	}

	if options.ChaptersFilter.IsPresent() {
		chapters, err = options.ChaptersFilter.MustGet()(chapters)
		if err != nil {
			return err
		}
	}

	if options.Json {
		if err = prepareManga(manga, options); err != nil {
			return err
		}

		marshalled, err := asJson([]*source.Manga{manga}, options)
		if err != nil {
			return err
		}

		_, err = options.Out.Write(marshalled)
		return err
	}

	for i := range chapters {
		if options.Download {
			path, err := downloader.Download(chapters[i], func(string) {})
			if err != nil {
				if viper.GetBool(key.DownloaderStopOnError) {
					return err
				}

				continue
			}

			_, err = options.Out.Write([]byte(path + "\n"))
			if err != nil {
				log.Warn(err)
			}
		} else {
			err := readWithPreload(chapters, i)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func readWithPreload(chapters source.Chapters, idx int) error {

	currChapter, _ := chapters.GetCurrentChapter(idx)

	go func() {
		nextChapter, err := chapters.GetNextChapter(idx, 1)
		if err != nil {
			log.Error(err)
			return
		}
		_ = downloader.Preload(nextChapter, func(string) {})
	}()

	err := downloader.Read(currChapter, func(string) {})
	if err != nil {
		return err
	}

	return nil
}
