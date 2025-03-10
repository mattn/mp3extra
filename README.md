# 🎧mp3extra

`mp3extra` is a simple CLI tool for embedding images and lyrics into MP3 files.

## ✨Features

- **🎨 Custom Album Art**: Easily add or automatically fetch stunning cover images.
- **🎤 Lyrics Embedding**:Insert lyrics directly into your MP3 files using ID3 tags.
- **⚡ Sleek CLI**: Enjoy a straightforward and user-friendly command-line interface.
- **🔄 Batch Processing**:Process multiple files
- **🤖 Auto Fetching**:Automatically retrieve images and lyrics for a hassle-free experience.

## 🛠️Installation

```sh
go install github.com/mattn/mp3extra@latest
```

## 🚀Usage

### Embed an image

```sh
mp3extra -image cover.jpg song.mp3
```

### Automatically fetch and embed an image

```sh
mp3extra -image auto song.mp3
```

### Embed lyrics

```sh
mp3extra -lyrics lyrics.lrc song.mp3
```

### Automatically fetch and embed lyrics

```sh
mp3extra -lyrics auto song.mp3
```

### Embed both image and lyrics

```sh
mp3extra -image cover.jpg -lyrics lyrics.lrc song.mp3
```

## 📜License

Released under the MIT License.see the [LICENSE](LICENSE) file for details.

## 👤Author

Created by Yasuhiro Matsumoto (a.k.a. mattn)

## ⭐Star History

[![Star History Chart](https://api.star-history.com/svg?repos=mattn/mp3extra&type=Date)](https://www.star-history.com/#mattn/mp3extra&Date)
