//go:build !windows || !embedded_media

package mediaruntime

func embeddedArchive() []byte { return nil }
