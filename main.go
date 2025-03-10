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

// lrclibResult represents a single result from the LRC lyrics API.
// It holds various metadata for a track as returned by the API.
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

// downloadLrc fetches synchronized lyrics from the LRC API for a given artist and title.
// It returns the synced lyrics if a matching record is found.
func downloadLrc(artist, title string) (string, error) {
	// Build the API URL with query parameters.
	resp, err := http.Get("https://lrclib.net/api/search?q=" + url.QueryEscape(artist+" "+title))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Decode the JSON response into a slice of lrclibResult.
	var results []lrclibResult
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return "", err
	}

	// Iterate through the results and return the synced lyrics for an exact match.
	for _, r := range results {
		if r.ArtistName == artist && r.TrackName == title {
			return r.SyncedLyrics, nil
		}
	}
	return "", fmt.Errorf("lyrics not found for %s - %s", artist, title)
}

// itunesResult represents the JSON structure returned by the iTunes API.
// It holds the results array containing album art information.
type itunesResult struct {
	Results []struct {
		ArtworkURL100 string `json:"artworkUrl100"`
	} `json:"results"`
}

// coverArtUrl constructs the iTunes API URL to search for album art using artist and title.
func coverArtUrl(artist, title string) string {
	return "https://itunes.apple.com/search?term=" + url.QueryEscape(artist+" "+title) + "&media=music&limit=1"
}

// fetchAlbumArtURL retrieves the album art image from the iTunes API.
// It first queries the API to get the artwork URL, then replaces the size to fetch a higher resolution image.
// Returns the image data, its content type, or an error.
func fetchAlbumArtURL(u string) ([]byte, string, error) {
	// First API call to fetch the artwork URL.
	resp, err := http.Get(u)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Decode the JSON response from iTunes.
	var result itunesResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}

	// Check if any result was returned.
	if len(result.Results) == 0 {
		return nil, "", fmt.Errorf("album art not found")
	}

	// Modify the URL to request a larger image (600x600 instead of 100x100).
	resp, err = http.Get(strings.Replace(result.Results[0].ArtworkURL100, "100x100", "600x600", 1))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Read the image bytes.
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return b, resp.Header.Get("content-type"), nil
}

// main is the entry point of the program. It parses command-line flags,
// opens the MP3 file, and conditionally embeds album art and lyrics based on the provided flags.
func main() {
	// Define command-line flags.
	var embedImage, embedLyrics, embedLang string
	var dryRun bool
	flag.StringVar(&embedImage, "image", "", "Path to image file to embed or 'auto' for automatic cover art fetch")
	flag.StringVar(&embedLyrics, "lyrics", "", "Path to lyrics file to embed or 'auto' for automatic lyrics fetch")
	flag.StringVar(&embedLang, "lang", "jpn", "Language code for embedded tag (e.g., jpn, eng)")
	flag.BoolVar(&dryRun, "dryrun", false, "Perform a dry run without modifying the file")
	flag.Parse()

	// Get the MP3 file from command-line arguments.
	mp3File := flag.Arg(0)
	if mp3File == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Open the MP3 file with ID3v2 tags.
	tag, err := id3v2.Open(mp3File, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatalf("Error opening MP3 file: %v", err)
	}
	defer tag.Close()

	// If dryRun is enabled, print out all current ID3v2 frames for review.
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
				// Switch on the type of frame to extract a summary string.
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
				// Truncate output to avoid overly long strings.
				if len(s) > 70 {
					s = s[:70] + "..."
				}
				s += " [" + reflect.TypeOf(v).Name() + "]"
				fmt.Printf("%v: %v\n", k, s)
			}
		}
	}

	// Set the default text encoding for added frames.
	tag.SetDefaultEncoding(id3v2.EncodingUTF16)

	// Normalize the comment tag to ensure compatibility with different tag editors.
	// Some editors do not handle multiple text encodings well.
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

	// Process embedding of album art if the image flag is provided.
	if embedImage != "" {
		// If "auto" is specified, automatically fetch album art via iTunes API.
		if embedImage == "auto" {
			u := coverArtUrl(tag.Artist(), tag.Title())
			if dryRun {
				fmt.Println()
				fmt.Println("Cover art URL:", u)
			} else {
				b, ct, err := fetchAlbumArtURL(u)
				if err != nil {
					log.Fatalf("Error fetching album art image: %v", err)
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
			// If a specific file path is provided, read and embed that image.
			if dryRun {
				fmt.Println()
				fmt.Println("Cover art from file:", embedImage)
			} else {
				b, err := os.ReadFile(embedImage)
				if err != nil {
					log.Fatalf("Error reading album art image: %v", err)
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

	// Process embedding of lyrics if the lyrics flag is provided.
	if embedLyrics != "" {
		// If "auto" is specified, automatically fetch lyrics using the LRC API.
		if embedLyrics == "auto" {
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
			// If a specific lyrics file path is provided, read and embed those lyrics.
			if dryRun {
				fmt.Println()
				fmt.Println("Lyrics text from file:", embedLyrics)
			} else {
				b, err := os.ReadFile(embedLyrics)
				if err != nil {
					log.Fatalf("Error reading lyrics file: %v", err)
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

	// If not a dry run, save the modified tags back to the MP3 file.
	if !dryRun {
		err = tag.Save()
		if err != nil {
			log.Fatalf("Error saving MP3 file: %v", err)
			return
		}
		fmt.Println("Embedded successfully in", mp3File)
	}
}
