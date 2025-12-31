package pl

type ImageError struct {
	Msg        string
	ReturnCode int
}

func NewImageError(cmdErr error, returncode int) *ImageError {
	return &ImageError{
		Msg:        cmdErr.Error(),
		ReturnCode: returncode,
	}
}

// func (i *ImageError) Error(msg string, returncode int) (int, string) {
// 	return i.ReturnCode, i.Msg
// }
