package main

import (
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type DictServer struct {
	index    *Index
	store    Store
	filepath string
	header   Header
}

func NewDictServer(index *Index, store Store, filepath string, header Header) *DictServer {
	return &DictServer{
		index:    index,
		store:    store,
		filepath: filepath,
		header:   header,
	}
}

func (s *DictServer) HandleLookup(c echo.Context) error  {
	word := strings.ToLower(strings.TrimSpace(c.Param("word")))

	if word == "" {

		return c.JSON(http.StatusBadRequest, map[string]string{"error": "empty word"})
	}
	entry, found := s.index.Lookup(word)
	if !found {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "word not found"})
	}

	absoluteOffset := int64(s.header.DataOffset) + entry.Offset
	data, err := s.store.ReadAt(s.filepath, absoluteOffset, entry.Length)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read defination"})
	}

	line := strings.TrimSpace(string(data))
	parts := strings.SplitN(line, ",", 2)
	meaning := ""
	if len(parts) == 2 {
		meaning = parts[1]
	}

	return c.JSON(http.StatusOK, map[string]string{"word": parts[0], "meaning": meaning})
 
}
