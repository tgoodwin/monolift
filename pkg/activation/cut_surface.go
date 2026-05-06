package activation

func classifySurface(step int, pathLen int) SurfaceClass {
	if pathLen <= 1 || step <= 0 {
		return VeryLarge
	}

	lastStep := pathLen - 1
	if pathLen <= 4 {
		switch {
		case step >= lastStep:
			return Minimal
		case step >= lastStep-1:
			return Small
		default:
			return Medium
		}
	}

	if step <= 1 {
		return VeryLarge
	}
	depth := float64(step) / float64(lastStep)
	switch {
	case depth < 0.25:
		return Large
	case depth < 0.50:
		return Medium
	case depth < 0.75:
		return Small
	default:
		return Minimal
	}
}
