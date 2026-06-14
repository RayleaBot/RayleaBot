package artifact

type Request struct {
	Template string
	Theme    string
	Output   string
}

type Result struct {
	ArtifactID string `json:"artifact_id"`
	ImagePath  string `json:"image_path"`
	MIME       string `json:"mime"`
	CacheKey   string `json:"cache_key"`
	Template   string `json:"template"`
	Theme      string `json:"theme"`
	FromCache  bool   `json:"from_cache"`
}

type Artifact struct {
	ArtifactID string
	MIME       string
	Path       string
}

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
