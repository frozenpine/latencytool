package libs

import "errors"

var (
	ErrInvalidPluginType = errors.New("invalid plugin type")

	ErrSetLoggerFailed = errors.New("set plugin logger failed")
	ErrInitFailed      = errors.New("initialize failed")
	ErrReportFailed    = errors.New("report fronts failed")
	ErrStopFailed      = errors.New("stop plugin failed")
	ErrJoinFailed      = errors.New("join exit failed")
)
