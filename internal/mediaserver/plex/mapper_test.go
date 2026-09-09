package plex

import (
	"testing"
	"time"

	"github.com/SuperCoolPencil/cue/internal/domain"
)

func TestMapPlexLibrariesFiltersSupportedTypes(t *testing.T) {
	libs := MapLibraries([]Directory{
		{Key: "1", Title: "Movies", Type: "movie", ContentChangedAt: 10},
		{Key: "2", Title: "Shows", Type: "show", ContentChangedAt: 20},
		{Key: "3", Title: "Music", Type: "artist"},
	})
	if len(libs) != 2 {
		t.Fatalf("libs = %#v", libs)
	}
	if libs[0].ID != "1" || libs[1].Type != "show" {
		t.Fatalf("mapped libs = %#v", libs)
	}
}

func TestMapPlexMovieEpisodeAndMixedContent(t *testing.T) {
	metadata := []Metadata{
		{
			RatingKey:             "m1",
			Type:                  "movie",
			Title:                 "Movie",
			LibrarySectionID:      7,
			Duration:              60000,
			ViewOffset:            1000,
			ContentRating:         "Unrated",
			AudienceRating:        8.4,
			Thumb:                 "/thumb",
			OriginallyAvailableAt: "2023-10-25",
			Media: []Media{{
				Bitrate:       9000,
				Width:         3840,
				Height:        2160,
				VideoCodec:    "h264",
				AudioCodec:    "dca",
				AudioChannels: 6,
				Container:     "mkv,mp4",
				Part:          []Part{{Size: 123}},
			}},
		},
		{RatingKey: "s1", Type: "show", Title: "Show", LeafCount: 10, ViewedLeafCount: 4},
		{RatingKey: "x", Type: "artist", Title: "Skip"},
	}

	movies := MapMovies(metadata, "http://server")
	if len(movies) != 1 {
		t.Fatalf("movies = %#v", movies)
	}
	movie := movies[0]
	if movie.LibraryID != "7" || movie.Duration != time.Minute || movie.ViewOffset != time.Second {
		t.Fatalf("movie timing/library = %#v", movie)
	}
	if movie.ContentRating != "NR" || movie.VideoCodec != "H.264" || movie.AudioCodec != "DTS" || movie.Container != "mkv" {
		t.Fatalf("movie normalization = %#v", movie)
	}
	if movie.ThumbURL != "http://server/thumb" || movie.FileSize != 123 || movie.AirDate != "2023-10-25" {
		t.Fatalf("movie media fields = %#v", movie)
	}

	mixed := MapLibraryContent(metadata, "http://server")
	if len(mixed) != 2 {
		t.Fatalf("mixed = %#v", mixed)
	}
	if _, ok := mixed[0].(*domain.MediaItem); !ok {
		t.Fatalf("first mixed type = %T", mixed[0])
	}
	if show, ok := mixed[1].(*domain.Show); !ok || show.UnwatchedCount != 6 {
		t.Fatalf("show = %#v type=%T", mixed[1], mixed[1])
	}
}

func TestMapPlexEpisodeAndPlaylist(t *testing.T) {
	episode := MapEpisodes([]Metadata{{
		RatingKey:             "e1",
		Type:                  "episode",
		Title:                 "Episode",
		GrandparentTitle:      "Show",
		GrandparentRatingKey:  "show",
		ParentRatingKey:       "season",
		ParentIndex:           2,
		Index:                 3,
		OriginallyAvailableAt: "2023-11-01",
	}}, "")[0]
	if episode.Type != domain.MediaTypeEpisode || episode.EpisodeCode() != "S02E03" || episode.ParentID != "season" || episode.AirDate != "2023-11-01" {
		t.Fatalf("episode = %#v", episode)
	}

	playlists := MapPlaylists([]Metadata{{RatingKey: "p1", Type: "playlist", Title: "Queue", LeafCount: 2, Duration: 120000}}, "")
	if len(playlists) != 1 || playlists[0].Duration != 2*time.Minute {
		t.Fatalf("playlists = %#v", playlists)
	}
}

func TestPlexFindBestMedia(t *testing.T) {
	mediaList := []Media{
		{Width: 1920, Height: 1080, Bitrate: 5000},
		{Width: 3840, Height: 2160, Bitrate: 20000},
		{Width: 1280, Height: 720, Bitrate: 2000},
	}

	best := findBestMedia(mediaList)
	if best.Width != 3840 {
		t.Errorf("findBestMedia() width = %d; want 3840", best.Width)
	}
}

func TestPlexMediaURLNormalizesRelativeAndAbsolutePaths(t *testing.T) {
	item := mapMovie(Metadata{
		RatingKey: "movie",
		Type:      "movie",
		Thumb:     "https://cdn.example/poster.jpg",
		Art:       "/library/art",
	}, "http://plex.example:32400/")

	if item.ThumbURL != "https://cdn.example/poster.jpg" {
		t.Fatalf("absolute poster URL = %q", item.ThumbURL)
	}
	if item.ArtURL != "http://plex.example:32400/library/art" {
		t.Fatalf("relative art URL = %q", item.ArtURL)
	}
}
