package log

import (
	"sync"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
)

type HandlerCreatorOptions struct {
	Path string
}

type HandlerCreator func(LogType, HandlerCreatorOptions) (log.Handler, error)

// handlerCreatorMap is the whole table: the log types this build can create a
// handler for, one line each. RegisterHandlerCreator is only for replacing an
// entry, e.g. in a test, so nothing registers itself from an init() function.
var handlerCreatorMap = map[LogType]HandlerCreator{
	LogType_Console: func(lt LogType, options HandlerCreatorOptions) (log.Handler, error) {
		return log.NewLogger(log.CreateStdoutLogWriter()), nil
	},
	LogType_File: func(lt LogType, options HandlerCreatorOptions) (log.Handler, error) {
		creator, err := log.CreateFileLogWriter(options.Path)
		if err != nil {
			return nil, err
		}
		return log.NewLogger(creator), nil
	},
	LogType_None: func(lt LogType, options HandlerCreatorOptions) (log.Handler, error) {
		return nil, nil
	},
}

var handlerCreatorMapLock = &sync.RWMutex{}

func RegisterHandlerCreator(logType LogType, f HandlerCreator) error {
	if f == nil {
		return errors.New("nil HandlerCreator")
	}

	handlerCreatorMapLock.Lock()
	defer handlerCreatorMapLock.Unlock()

	handlerCreatorMap[logType] = f
	return nil
}

func createHandler(logType LogType, options HandlerCreatorOptions) (log.Handler, error) {
	handlerCreatorMapLock.RLock()
	defer handlerCreatorMapLock.RUnlock()

	creator, found := handlerCreatorMap[logType]
	if !found {
		return nil, errors.New("unable to create log handler for ", logType)
	}
	return creator(logType, options)
}
