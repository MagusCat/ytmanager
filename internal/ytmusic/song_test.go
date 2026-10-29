package ytmusic

import (
	"context"
	"errors"
	"testing"
)

func track(id, kind string) map[string]any {
	return map[string]any{
		"videoId": id,
		"navigationEndpoint": map[string]any{"watchEndpoint": map[string]any{
			"watchEndpointMusicSupportedConfigs": map[string]any{
				"watchEndpointMusicConfig": map[string]any{"musicVideoType": kind},
			},
		}},
	}
}

func panel(t map[string]any) map[string]any {
	return map[string]any{"playlistPanelVideoRenderer": t}
}

func pair(primary, other map[string]any) map[string]any {
	return map[string]any{"playlistPanelVideoWrapperRenderer": map[string]any{
		"primaryRenderer": panel(primary),
		"counterpart":     []any{map[string]any{"counterpartRenderer": panel(other)}},
	}}
}

func TestSongIdentity(t *testing.T) {
	audio := track("audio", "MUSIC_VIDEO_TYPE_ATV")
	video := track("video", "MUSIC_VIDEO_TYPE_OMV")
	blocked := track("audio", "MUSIC_VIDEO_TYPE_ATV")
	blocked["unplayableText"] = map[string]any{"simpleText": "Restricted"}
	for _, tc := range []struct {
		name     string
		input    any
		id, want string
	}{
		{"already audio", panel(audio), "audio", "audio"},
		{"video to song", pair(video, audio), "video", "audio"},
		{"song primary", pair(audio, video), "video", "audio"},
		{"never reverse", pair(audio, video), "audio", "audio"},
		{"unrelated recommendation", []any{panel(audio), panel(video)}, "video", ""},
		{"unrelated pair", pair(audio, video), "other", ""},
		{"untyped counterpart", pair(video, track("audio", "")), "video", ""},
		{"blocked counterpart", pair(video, blocked), "video", ""},
		{"ambiguous", []any{pair(video, audio), pair(video, track("other", "MUSIC_VIDEO_TYPE_ATV"))}, "video", ""},
		{"malformed", map[string]any{"error": "bad request"}, "video", ""},
		{"empty id", panel(audio), "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := songFromResponse(tc.input, tc.id)
			if got != tc.want || (tc.want == "" && !errors.Is(err, ErrNoSong)) || (tc.want != "" && err != nil) {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSongCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Song(ctx, "video")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
