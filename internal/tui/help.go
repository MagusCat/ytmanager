package tui

func (m model) help() string {
	switch m.screen {
	case screenDevices:
		help := "↑↓ move · enter browse · e edit path · paste a path · r refresh · q quit"
		if m.lost != nil {
			help = "x stop waiting · " + help
		}
		return help
	case screenBrowse:
		return "↑↓ move · → open · ← up · u use · n new folder · e edit path · d devices · esc back"
	case screenSettings:
		return "↑↓ move · ←→ change · esc back"
	case screenMissing:
		return "↑↓ move · paste or enter alternative · r retry · esc back"
	default:
		if m.run.busy {
			return "↑↓ sources · paste or a add · p pause/resume · x cancel · n unavailable · o settings · q quit"
		}
		return "↑↓ sources · a add · enter sync · space mark · esc/c folder · d remove · n unavailable · o settings · q quit"
	}
}
