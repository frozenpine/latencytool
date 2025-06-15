package libs

import "errors"

var (
	ErrInitFailed   = errors.New("initialize failed")
	ErrReportFailed = errors.New("report fronts failed")
	ErrStopFailed   = errors.New("stop plugin failed")
	ErrJoinFailed   = errors.New("join exit failed")
)
