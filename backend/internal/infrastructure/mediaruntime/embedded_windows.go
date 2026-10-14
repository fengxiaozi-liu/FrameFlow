//go:build windows && embedded_media

package mediaruntime

import _ "embed"

//go:embed assets/media-tools.zip
var archive []byte

func embeddedArchive() []byte { return archive }
