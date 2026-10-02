package shipper

import (
	"path"
	"slices"
	"time"

	"github.com/markusheinemann/downline/packages/archive"
)

type upload struct {
	LocalName  string
	RemotePath string
	Hour       time.Time
}

func plan(names []string, now time.Time, remoteRoot string) (uploads []upload, ignored []string) {
	currentHour := now.UTC().Truncate(time.Hour)

	for _, name := range names {
		parsedTime, err := archive.ParseFileName(name)
		if err != nil {
			ignored = append(ignored, name)
			continue
		}

		if !parsedTime.Before(currentHour) {
			continue
		}

		uploads = append(uploads, upload{
			LocalName:  name,
			RemotePath: path.Join(remoteRoot, parsedTime.Format("2006/01/02"), name),
			Hour:       parsedTime,
		})
	}

	slices.SortFunc(uploads, func(a, b upload) int {
		return a.Hour.Compare(b.Hour)
	})

	return uploads, ignored
}
