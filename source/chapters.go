package source

import "fmt"

type Chapters []*Chapter

var (
	ErrOutOfBounds = fmt.Errorf("out of bounds")
)

func (c Chapters) GetCurrentChapter(curr int) (*Chapter, error) {
	if curr < 0 || curr >= len(c) {
		return nil, ErrOutOfBounds
	}

	return c[curr], nil
}

func (c Chapters) GetNextChapter(curr int, next uint16) (*Chapter, error) {
	chp, err := c.GetCurrentChapter(curr)
	if err != nil {
		return nil, err
	}

	if int(chp.Index+next) >= len(c) {
		return nil, ErrOutOfBounds
	}

	return c[int(chp.Index+next)], nil
}

func (c Chapters) GetPrevChapter(curr int, prev uint16) (*Chapter, error) {
	chp, err := c.GetCurrentChapter(curr)
	if err != nil {
		return nil, err
	}

	if int(chp.Index-prev) < 0 {
		return nil, ErrOutOfBounds
	}

	return c[int(chp.Index-prev)], nil
}
