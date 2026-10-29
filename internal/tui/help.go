package tui

func (m model) help() string {
	switch m.screen {
	case screenDevices:
		help := "↑↓ move · enter open · e edit path · paste path · r refresh · q quit"
		if m.lost != nil {
			help = "x stop waiting · " + help
		}
		return help
	case screenBrowse:
		return "↑↓ move · → open · ← up · u use · n new folder · e edit path · d devices · esc back"
	case screenSettings:
		return "↑↓ move · ←→ change · esc back"
	case screenLibrary:
		if m.library.busy {
			return "Deleting files…"
		}
		if len(m.library.confirm) > 0 {
			return "enter delete permanently · esc cancel"
		}
		if m.library.search {
			return "type to search · enter apply · esc cancel"
		}
		if m.library.scanning {
			return "Reading files and comparing SHA-256… · x cancel"
		}
		return "s scan phone · / search · f duplicates · space mark · a mark results · d delete · esc back"
	case screenMissing:
		return "↑↓ move · paste or enter alternative · r retry · esc back"
	default:
		if m.run.busy && (m.run.verb == "repaired" || m.run.verb == "exported") {
			return "Checking library… · x cancel · q quit"
		}
		if m.run.busy {
			return "↑↓ move · a/paste add · p pause · x cancel · n unavailable · o settings · q quit"
		}
		return "enter resume · s sync · a add · space mark · d remove · l library · r repair · e export · n unavailable · o settings · c folder · q quit"
	}
}
