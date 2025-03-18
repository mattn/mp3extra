package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/bogem/id3v2/v2"
)

type lrclibResult struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

func downloadLrc(artist, title string) (string, error) {
	resp, err := http.Get("https://lrclib.net/api/search?q=" + url.QueryEscape(artist+" "+title))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var results []lrclibResult
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return "", err
	}
	for _, r := range results {
		if r.ArtistName == artist && r.TrackName == title {
			return r.SyncedLyrics, nil
		}
	}
	return "", fmt.Errorf("Lyrics not found for %s - %s", artist, title)
}

type itunesResult struct {
	Results []struct {
		ArtworkURL100 string `json:"artworkUrl100"`
	} `json:"results"`
}

func coverArtUrl(artist, title string) string {
	return "https://itunes.apple.com/search?term=" + url.QueryEscape(artist+" "+title) + "&media=music&limit=1"
}

func fetchAlbumArtURL(u string) ([]byte, string, error) {
	resp, err := http.Get(u)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	var result itunesResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}

	if len(result.Results) == 0 {
		return nil, "", fmt.Errorf("Album art not found")
	}

	resp, err = http.Get(strings.Replace(result.Results[0].ArtworkURL100, "100x100", "600x600", 1))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return b, resp.Header.Get("content-type"), nil
}

func main() {
	var embedImage string
	var embedLyrics string
	var embedLang string
	var dryRun bool
	flag.StringVar(&embedImage, "image", "", "embed image")
	flag.StringVar(&embedLyrics, "lyrics", "", "embed lyrics")
	flag.StringVar(&embedLang, "lang", "jpn", "language for embed tag")
	flag.BoolVar(&dryRun, "dryrun", false, "dry run")
	flag.Parse()

	mp3File := flag.Arg(0)
	if mp3File == "" {
		flag.Usage()
		os.Exit(1)
	}

	tag, err := id3v2.Open(mp3File, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatalf("error opening MP3 file: %v", err)
	}
	defer tag.Close()

	if dryRun {
		frames := tag.AllFrames()
		var ks []string
		for k := range frames {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			for _, v := range frames[k] {
				var s string
				switch t := v.(type) {
				case id3v2.TextFrame:
					s = t.Text
				case id3v2.CommentFrame:
					s = t.Text
				case id3v2.PictureFrame:
					s = t.Description
				default:
					s = fmt.Sprint(v)
				}
				if len(s) > 70 {
					s = s[:70] + "..."
				}
				s += " [" + reflect.TypeOf(v).Name() + "]"
				fmt.Printf("%v: %v\n", k, s)
			}
		}
	}

	tag.SetDefaultEncoding(id3v2.EncodingUTF16)

	// fix comment tag: id3v2 normalize tags. Some tag editor does not handle texts with multiple text encoding.
	comments := tag.GetFrames(tag.CommonID("Comments"))
	if len(comments) > 0 {
		comment := id3v2.CommentFrame{
			Encoding:    id3v2.EncodingISO,
			Language:    embedLang,
			Description: comments[0].(id3v2.CommentFrame).Description,
			Text:        comments[0].(id3v2.CommentFrame).Text,
		}
		tag.DeleteFrames(tag.CommonID("Comments"))
		tag.AddCommentFrame(comment)
	}

	if embedImage != "" {
		if embedImage == "auto" {
			u := coverArtUrl(tag.Artist(), tag.Title())
			if dryRun {
				fmt.Println()
				fmt.Println("Covert art:", u)
			} else {
				b, ct, err := fetchAlbumArtURL(u)
				if err != nil {
					log.Fatalf("error fetch album art image: %v", err)
				}
				pic := id3v2.PictureFrame{
					Encoding:    id3v2.EncodingISO,
					MimeType:    ct,
					PictureType: id3v2.PTFrontCover,
					Description: "Cover Art",
					Picture:     b,
				}
				tag.DeleteFrames(tag.CommonID("Attached picture"))
				tag.AddAttachedPicture(pic)
			}
		} else {
			if dryRun {
				fmt.Println()
				fmt.Println("Covert art:", embedImage)
			} else {
				b, err := os.ReadFile(embedImage)
				if err != nil {
					log.Fatalf("error read album art image: %v", err)
				}
				ct := http.DetectContentType(b)
				pic := id3v2.PictureFrame{
					Encoding:    id3v2.EncodingISO,
					MimeType:    ct,
					PictureType: id3v2.PTFrontCover,
					Description: "Cover Art",
					Picture:     b,
				}
				tag.DeleteFrames(tag.CommonID("Attached picture"))
				tag.AddAttachedPicture(pic)
			}
		}
	}

	if embedLyrics != "" {
		if embedImage == "auto" {
			lyrics, err := downloadLrc(tag.Artist(), tag.Title())
			if err != nil {
				log.Fatal(err)
			}
			if dryRun {
				fmt.Println()
				fmt.Println(lyrics)
			} else {
				uslt := id3v2.UnsynchronisedLyricsFrame{
					Encoding:          id3v2.EncodingUTF8,
					Language:          embedLang,
					ContentDescriptor: "Lyrics",
					Lyrics:            lyrics,
				}
				tag.DeleteFrames(tag.CommonID("Unsynchronised lyrics/text transcription"))
				tag.AddUnsynchronisedLyricsFrame(uslt)
			}
		} else {
			if dryRun {
				fmt.Println()
				fmt.Println("Lyrics text:", embedLyrics)
			} else {
				b, err := os.ReadFile(embedLyrics)
				if err != nil {
					log.Fatalf("error read lyrics file: %v", err)
				}
				uslt := id3v2.UnsynchronisedLyricsFrame{
					Encoding:          id3v2.EncodingUTF8,
					Language:          embedLang,
					ContentDescriptor: "Lyrics",
					Lyrics:            string(b),
				}
				tag.DeleteFrames(tag.CommonID("Unsynchronised lyrics/text transcription"))
				tag.AddUnsynchronisedLyricsFrame(uslt)
			}
		}
	}

	if !dryRun {
		err = tag.Save()
		if err != nil {
			log.Fatalf("error saving MP3 file: %v", err)
			return
		}

		fmt.Println("Embedded successfully in", mp3File)
	}
}
