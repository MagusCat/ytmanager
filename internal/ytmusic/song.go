package ytmusic

import (
	"context"
	"errors"
)

// ErrNoSong is deliberately not a permanent/unavailable-video error: the user
// can retry or choose a specific recording using the existing alternative flow.
var ErrNoSong = errors.New("no linked YouTube Music audio recording could be confirmed; choose a song link as an alternative")

// Song only accepts an audio track tied to the requested ID. Recommendations,
// title searches and the first item in a radio queue are not identity evidence.
// This uses YouTube Music's unofficial next endpoint, as does lyrics lookup.
func Song(ctx context.Context, id string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body := map[string]any{
		"context": map[string]any{"client": map[string]any{
			"clientName": "WEB_REMIX", "clientVersion": clientVersion(ctx), "hl": "en",
		}},
		"videoId": id, "playlistId": "RDAMVM" + id,
		"isAudioOnly": true, "enablePersistentPlaylistPanel": true,
		"watchEndpointMusicSupportedConfigs": map[string]any{
			"watchEndpointMusicConfig": map[string]any{
				"hasPersistentPlaylistPanel": true, "musicVideoType": "MUSIC_VIDEO_TYPE_ATV",
			},
		},
	}
	response, err := innertube(ctx, "next", body)
	if err != nil {
		return "", err
	}
	return songFromResponse(response, id)
}

func field(v any, keys ...string) any {
	for _, k := range keys {
		m, _ := v.(map[string]any)
		v = m[k]
	}
	return v
}

func text(v any) string { s, _ := v.(string); return s }

func audioID(v any) string {
	if field(v, "unplayableText") != nil || field(v, "isPlayable") == false {
		return ""
	}
	kind := field(v, "navigationEndpoint", "watchEndpoint", "watchEndpointMusicSupportedConfigs", "watchEndpointMusicConfig", "musicVideoType")
	if kind != "MUSIC_VIDEO_TYPE_ATV" {
		return ""
	}
	return text(field(v, "videoId"))
}

func songFromResponse(response any, id string) (string, error) {
	candidates := map[string]bool{}
	var visit func(any)
	visit = func(v any) {
		switch node := v.(type) {
		case []any:
			for _, child := range node {
				visit(child)
			}
		case map[string]any:
			if wrapper, ok := node["playlistPanelVideoWrapperRenderer"]; ok {
				group := []any{field(wrapper, "primaryRenderer", "playlistPanelVideoRenderer")}
				counterparts, _ := field(wrapper, "counterpart").([]any)
				for _, c := range counterparts {
					group = append(group, field(c, "counterpartRenderer", "playlistPanelVideoRenderer"))
				}
				linked := false
				for _, track := range group {
					linked = linked || text(field(track, "videoId")) == id
				}
				if linked {
					for _, track := range group {
						if candidate := audioID(track); candidate != "" {
							candidates[candidate] = true
						}
					}
				}
				return
			}
			if track, ok := node["playlistPanelVideoRenderer"]; ok {
				if audioID(track) == id {
					candidates[id] = true
				}
				return
			}
			for _, child := range node {
				visit(child)
			}
		}
	}
	if id == "" {
		return "", ErrNoSong
	}
	visit(response)
	if candidates[id] {
		return id, nil
	}
	if len(candidates) == 1 {
		for candidate := range candidates {
			return candidate, nil
		}
	}
	return "", ErrNoSong
}
