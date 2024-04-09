package downloader

import (
	"fmt"

	"github.com/metafates/gache"
	"github.com/metafates/mangal/color"
	"github.com/metafates/mangal/constant"
	"github.com/metafates/mangal/converter"
	"github.com/metafates/mangal/filesystem"
	"github.com/metafates/mangal/history"
	"github.com/metafates/mangal/key"
	"github.com/metafates/mangal/log"
	"github.com/metafates/mangal/open"
	"github.com/metafates/mangal/source"
	"github.com/metafates/mangal/style"
	"github.com/metafates/mangal/where"
	"github.com/spf13/viper"
)

// Key: encoded chapter
// Value: tmp file path for the chapter
var cacher = gache.New[map[string]string](
	&gache.Options{
		Path:       where.Loaded(),
		FileSystem: &filesystem.GacheFs{},
	},
)

// Read the chapter by downloading it with the given source
// and opening it with the configured reader.
func Read(chapter *source.Chapter, progress func(string)) error {

	if viper.GetBool(key.ReaderReadInBrowser) {
		return open.StartWith(
			chapter.URL,
			viper.GetString(key.ReaderBrowser),
		)
	}

	path, err := loadChapter(chapter, progress)
	if err != nil {
		log.Error(err)
		return err
	}

	err = openRead(path, chapter, progress)
	if err != nil {
		log.Error(err)
		return err
	}

	progress("Done")
	return nil
}

func loadChapter(chapter *source.Chapter, progress func(string)) (string, error) {

	// First option: try to read from cache first
	key := encodeChapterKey(chapter)
	loaded, err := get(key)
	if err == nil {
		exist, err := filesystem.Api().Exists(loaded)
		if err == nil && exist {
			log.Info("find loaded chapter in cache")
			progress("Load from cache")
			return loaded, nil
		}
	}

	// Last / fallback option: read directly from source
	path, err := fetchDirectlyFromSource(chapter, progress)
	if err != nil {
		log.Error(err)
		return "", err
	}
	_ = save(key, path)

	return path, nil
}

func fetchDirectlyFromSource(chapter *source.Chapter, progress func(string)) (string, error) {
	if viper.GetBool(key.DownloaderReadDownloaded) && chapter.IsDownloaded() {
		path, err := chapter.Path(false)
		if err == nil {
			return path, nil
		}
	}

	log.Infof("downloading %s for reading. Provider is %s", chapter.Name, chapter.Source().ID())
	log.Infof("getting pages of %s", chapter.Name)
	progress("Getting pages")
	pages, err := chapter.Source().PagesOf(chapter)
	if err != nil {
		log.Error(err)
		return "", err
	}

	err = chapter.DownloadPages(true, progress)
	if err != nil {
		log.Error(err)
		return "", err
	}

	log.Info("getting " + viper.GetString(key.FormatsUse) + " converter")
	conv, err := converter.Get(viper.GetString(key.FormatsUse))
	if err != nil {
		log.Error(err)
		return "", err
	}

	log.Info("converting " + viper.GetString(key.FormatsUse))
	progress(fmt.Sprintf(
		"Converting %d pages to %s %s",
		len(pages),
		style.Fg(color.Yellow)(viper.GetString(key.FormatsUse)),
		style.Faint(chapter.SizeHuman())),
	)
	path, err := conv.SaveTemp(chapter)
	if err != nil {
		log.Error(err)
		return "", err
	}

	return path, nil
}

func openRead(path string, chapter *source.Chapter, progress func(string)) error {
	if viper.GetBool(key.HistorySaveOnRead) {
		go func() {
			err := history.Save(chapter)
			if err != nil {
				log.Warn(err)
			} else {
				log.Info("history saved")
			}
		}()
	}

	var (
		reader string
		err    error
	)

	switch viper.GetString(key.FormatsUse) {
	case constant.FormatPDF:
		reader = viper.GetString(key.ReaderPDF)
	case constant.FormatCBZ:
		reader = viper.GetString(key.ReaderCBZ)
	case constant.FormatZIP:
		reader = viper.GetString(key.ReaderZIP)
	case constant.FormatPlain:
		reader = viper.GetString(key.RaderPlain)
	}

	if reader != "" {
		log.Info("opening with " + reader)
		progress(fmt.Sprintf("Opening %s", reader))
	} else {
		log.Info("no reader specified. opening with default")
		progress("Opening")
	}

	err = open.RunWith(path, reader)
	if err != nil {
		log.Error(err)
		return fmt.Errorf("could not open %s with %s: %s", path, reader, err.Error())
	}

	log.Info("opened without errors")

	return nil
}

// get returns all loaded chapters information + tmp file location from the loaded file
func get(key string) (filepath string, err error) {
	cached, expired, err := cacher.Get()
	if err != nil {
		return "", err
	}

	if expired || cached == nil {
		return "", fmt.Errorf("expired cache") // TODO: check if we had file containing all possible error
	}

	if _, ok := cached[key]; !ok {
		return "", fmt.Errorf("file not loaded")
	}

	return cached[key], nil
}

// save saves the chapter to the history file
func save(key string, val string) error {
	cached, expired, err := cacher.Get()
	if err != nil {
		return err
	}

	if expired || cached == nil {
		cached = make(map[string]string)
	}

	cached[key] = val
	return cacher.Set(cached)
}

func encodeChapterKey(c *source.Chapter) string {
	var sourceName string
	if c.Source() != nil {
		sourceName = c.Source().Name()
	}
	return fmt.Sprintf("%s-%s-%d-%s-%d-%s-%s",
		c.Manga,
		c.Name,
		c.Index,
		fmt.Sprintf("%04d", c.Index),
		len(c.Manga.Chapters),
		c.Volume,
		sourceName,
	)
}
