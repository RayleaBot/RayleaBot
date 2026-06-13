package bridge

func emitObservabilityFrame(subscriber chan ObservabilityFrame, frame ObservabilityFrame) {
	select {
	case subscriber <- frame:
	default:
		select {
		case <-subscriber:
		default:
		}
		select {
		case subscriber <- frame:
		default:
		}
	}
}
