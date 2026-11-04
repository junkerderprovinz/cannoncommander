package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type sideData struct {
	Type     string  `json:"side_data_type"`
	Rotation float64 `json:"rotation"`
}

type probe struct {
	Streams []struct {
		CodecName     string            `json:"codec_name"`
		CodecType     string            `json:"codec_type"`
		Profile       string            `json:"profile"`
		Width         int64             `json:"width"`
		Height        int64             `json:"height"`
		DisplayAspect string            `json:"display_aspect_ratio"`
		AvgFrameRate  string            `json:"avg_frame_rate"`
		RFrameRate    string            `json:"r_frame_rate"`
		BitRate       string            `json:"bit_rate"`
		SampleRate    string            `json:"sample_rate"`
		Channels      int               `json:"channels"`
		ChannelLayout string            `json:"channel_layout"`
		ColorTransfer string            `json:"color_transfer"`
		Duration      string            `json:"duration"`
		Disposition   map[string]int    `json:"disposition"`
		Tags          map[string]string `json:"tags"`
		SideData      []sideData        `json:"side_data_list"`
	} `json:"streams"`
	Format struct {
		Duration string            `json:"duration"`
		BitRate  string            `json:"bit_rate"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
}

func readAV(ctx context.Context, s *Service, f File, rec Record) error {
	ffprobe, ok := s.tools.find("ffprobe")
	if !ok {
		return errNoTool
	}
	out, err := run(ctx, ffprobe, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", "--", f.Real)
	if err != nil && len(out) == 0 {
		return err
	}
	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		return fmt.Errorf("ffprobe output: %w", err)
	}
	fillAV(&p, rec)
	return nil
}

func fillAV(p *probe, rec Record) {
	duration := number(p.Format.Duration)
	var audioRates int64
	var audio, subs []string
	videoSeen, audioSeen := false, false
	for _, st := range p.Streams {
		switch st.CodecType {
		case "video":
			if videoSeen || st.Disposition["attached_pic"] == 1 || st.Width == 0 {
				continue
			}
			videoSeen = true
			w, h := st.Width, st.Height
			for _, sd := range st.SideData {
				if r := math.Abs(sd.Rotation); r == 90 || r == 270 {
					w, h = h, w
				}
			}
			if r := math.Abs(number(tag(st.Tags, "rotate"))); r == 90 || r == 270 {
				w, h = h, w
			}
			rec["width"], rec["height"] = w, h
			rec["vcodec"] = videoCodec(st.CodecName)
			if fps := rate(st.AvgFrameRate); fps > 0 {
				rec["framerate"] = decimal(fps, 3) + " fps"
			} else if fps := rate(st.RFrameRate); fps > 0 && fps < 1000 {
				rec["framerate"] = decimal(fps, 3) + " fps"
			}
			if br := streamRate(st.BitRate, st.Tags); br > 0 {
				rec["vbitrate"] = int64(math.Round(br / 1000))
			}
			if a := aspect(st.DisplayAspect, w, h); a != "" {
				rec["aspect"] = a
			}
			rec["hdr"] = hdrOf(st.ColorTransfer, st.SideData)
			if duration == 0 {
				duration = number(st.Duration)
			}
		case "audio":
			br := streamRate(st.BitRate, st.Tags)
			audioRates += int64(br)
			audio = append(audio, language(tag(st.Tags, "language")))
			if audioSeen {
				continue
			}
			audioSeen = true
			rec["acodec"] = audioCodec(st.CodecName, st.Profile)
			if br == 0 && !videoSeen {
				br = number(p.Format.BitRate)
			}
			if br > 0 {
				rec["abitrate"] = fmt.Sprintf("%d kBit/s", int64(math.Round(br/1000)))
			}
			if sr := number(st.SampleRate); sr > 0 {
				rec["samplerate"] = decimal(sr/1000, 1) + " kHz"
			}
			if c := channels(st.Channels, st.ChannelLayout); c != "" {
				rec["channels"] = c
			}
			if duration == 0 {
				duration = number(st.Duration)
			}
		case "subtitle":
			subs = append(subs, language(tag(st.Tags, "language")))
		}
	}
	if _, ok := rec["vbitrate"]; !ok && videoSeen {
		if total := number(p.Format.BitRate); total > float64(audioRates) {
			rec["vbitrate"] = int64(math.Round((total - float64(audioRates)) / 1000))
		}
	}
	if duration > 0 {
		rec["duration"] = int64(math.Round(duration))
	}
	if videoSeen {
		rec["audiotracks"] = joinUnique(audio)
		rec["subtitles"] = joinUnique(subs)
	}
	rec["title"] = tag(p.Format.Tags, "title")
	rec["artist"] = tag(p.Format.Tags, "artist")
	rec["album"] = tag(p.Format.Tags, "album")
	if y := yearOf(tag(p.Format.Tags, "date")); y > 0 {
		rec["year"] = y
	} else if y := yearOf(tag(p.Format.Tags, "year")); y > 0 {
		rec["year"] = y
	}
}

// tag looks a tag up without minding case, since containers differ in
// how they spell them.
func tag(tags map[string]string, key string) string {
	if v, ok := tags[key]; ok {
		return strings.TrimSpace(v)
	}
	for k, v := range tags {
		if strings.EqualFold(k, key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func number(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// rate reads a frame rate such as 24000/1001.
func rate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return number(s)
	}
	d := number(den)
	if d == 0 {
		return 0
	}
	return number(num) / d
}

// streamRate takes the bit rate of a stream, or the BPS tag Matroska
// writes when the stream itself has none.
func streamRate(s string, tags map[string]string) float64 {
	if v := number(s); v > 0 {
		return v
	}
	for k, v := range tags {
		if k == "BPS" || strings.HasPrefix(k, "BPS-") {
			return number(v)
		}
	}
	return 0
}

func yearOf(s string) int64 {
	if len(s) < 4 {
		return 0
	}
	y, err := strconv.ParseInt(s[:4], 10, 64)
	if err != nil || y < 1000 {
		return 0
	}
	return y
}

var videoCodecs = map[string]string{
	"h264": "H.264", "hevc": "HEVC", "av1": "AV1", "vp9": "VP9", "vp8": "VP8", "mpeg4": "MPEG-4", "msmpeg4v3": "MPEG-4",
	"mpeg2video": "MPEG-2", "mpeg1video": "MPEG-1", "vc1": "VC-1", "wmv3": "WMV", "prores": "ProRes", "theora": "Theora",
	"mjpeg": "MJPEG", "h263": "H.263", "vvc": "VVC",
}

func videoCodec(name string) string {
	if v, ok := videoCodecs[name]; ok {
		return v
	}
	return strings.ToUpper(name)
}

var audioCodecs = map[string]string{
	"aac": "AAC", "ac3": "AC-3", "eac3": "E-AC-3", "dts": "DTS", "truehd": "TrueHD", "opus": "Opus", "vorbis": "Vorbis",
	"flac": "FLAC", "mp3": "MP3", "mp2": "MP2", "alac": "ALAC", "wmav2": "WMA", "wmapro": "WMA Pro",
}

func audioCodec(name, profile string) string {
	if name == "dts" && strings.HasPrefix(profile, "DTS-HD") {
		return profile
	}
	if strings.HasPrefix(name, "pcm_") {
		return "PCM"
	}
	if v, ok := audioCodecs[name]; ok {
		return v
	}
	return strings.ToUpper(name)
}

func channels(n int, layout string) string {
	switch {
	case strings.HasPrefix(layout, "7.1"):
		return "7.1"
	case strings.HasPrefix(layout, "5.1"):
		return "5.1"
	}
	switch n {
	case 0:
		return ""
	case 1:
		return "Mono"
	case 2:
		return "Stereo"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	}
	return fmt.Sprintf("%d Kanäle", n)
}

// aspect names the display shape the way film people do and snaps the
// odd pixel counts of real files to the ratio they aim at.
func aspect(dar string, w, h int64) string {
	r := rate(strings.Replace(dar, ":", "/", 1))
	if r <= 0 && w > 0 && h > 0 {
		r = float64(w) / float64(h)
	}
	if r <= 0 {
		return ""
	}
	for _, c := range []struct {
		r    float64
		name string
	}{{4.0 / 3, "4:3"}, {16.0 / 9, "16:9"}, {1, "1:1"}, {1.5, "3:2"}, {9.0 / 16, "9:16"}, {1.85, "1,85:1"}} {
		if math.Abs(r-c.r)/c.r < 0.03 {
			return c.name
		}
	}
	if r > 2.3 && r < 2.45 {
		return "21:9"
	}
	return decimal(r, 2) + ":1"
}

func hdrOf(transfer string, side []sideData) string {
	for _, sd := range side {
		if strings.Contains(sd.Type, "DOVI") {
			return "Dolby Vision"
		}
	}
	for _, sd := range side {
		if strings.Contains(sd.Type, "SMPTE2094-40") {
			return "HDR10+"
		}
	}
	switch transfer {
	case "smpte2084":
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	}
	return "SDR"
}

var languages = map[string]string{
	"deu": "Deutsch", "ger": "Deutsch", "de": "Deutsch", "eng": "Englisch", "en": "Englisch",
	"fra": "Französisch", "fre": "Französisch", "fr": "Französisch", "spa": "Spanisch", "es": "Spanisch",
	"ita": "Italienisch", "it": "Italienisch", "por": "Portugiesisch", "pt": "Portugiesisch",
	"nld": "Niederländisch", "dut": "Niederländisch", "nl": "Niederländisch", "pol": "Polnisch", "pl": "Polnisch",
	"ces": "Tschechisch", "cze": "Tschechisch", "cs": "Tschechisch", "slk": "Slowakisch", "slo": "Slowakisch", "sk": "Slowakisch",
	"hun": "Ungarisch", "hu": "Ungarisch", "rus": "Russisch", "ru": "Russisch", "ukr": "Ukrainisch", "uk": "Ukrainisch",
	"jpn": "Japanisch", "ja": "Japanisch", "kor": "Koreanisch", "ko": "Koreanisch", "zho": "Chinesisch", "chi": "Chinesisch", "zh": "Chinesisch",
	"tur": "Türkisch", "tr": "Türkisch", "swe": "Schwedisch", "sv": "Schwedisch", "nor": "Norwegisch", "nob": "Norwegisch", "nno": "Norwegisch", "no": "Norwegisch",
	"dan": "Dänisch", "da": "Dänisch", "fin": "Finnisch", "fi": "Finnisch", "ell": "Griechisch", "gre": "Griechisch", "el": "Griechisch",
	"hrv": "Kroatisch", "hr": "Kroatisch", "srp": "Serbisch", "sr": "Serbisch", "slv": "Slowenisch", "sl": "Slowenisch",
	"ron": "Rumänisch", "rum": "Rumänisch", "ro": "Rumänisch", "bul": "Bulgarisch", "bg": "Bulgarisch", "ara": "Arabisch", "ar": "Arabisch",
	"heb": "Hebräisch", "he": "Hebräisch", "hin": "Hindi", "hi": "Hindi", "tha": "Thailändisch", "th": "Thailändisch",
	"vie": "Vietnamesisch", "vi": "Vietnamesisch", "ind": "Indonesisch", "id": "Indonesisch", "cat": "Katalanisch", "ca": "Katalanisch",
	"lat": "Latein", "la": "Latein",
}

func language(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" || code == "und" {
		return "Unbekannt"
	}
	if name, ok := languages[code]; ok {
		return name
	}
	return strings.ToUpper(code)
}

func joinUnique(list []string) string {
	var out []string
	for _, s := range list {
		if !containsField(out, s) {
			out = append(out, s)
		}
	}
	return strings.Join(out, ", ")
}
