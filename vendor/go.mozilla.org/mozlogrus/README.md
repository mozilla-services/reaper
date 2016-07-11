# mozlogrus [![GoDoc](https://godoc.org/go.mozilla.org/mozlogrus?status.svg)](https://godoc.org/go.mozilla.org/mozlogrus)
A logging library which conforms to Mozilla's logging standard for [logrus](https://github.com/Sirupsen/logrus).

## Example Usage
```
import "go.mozilla.org/mozlogrus"

func init() {
    mozlogrus.Enable("ApplicationName")
}
```
