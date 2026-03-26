package llm

import "context"

type staticErrorResponder struct {
	err error
}

func NewStaticErrorResponder(err error) Responder {
	if err == nil {
		err = ErrResponderUnavailable
	}
	return staticErrorResponder{err: err}
}

func (s staticErrorResponder) Generate(context.Context, string) (string, error) {
	return "", s.err
}
