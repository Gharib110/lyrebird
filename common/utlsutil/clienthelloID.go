package utlsutil

import (
	"fmt"
	utls "github.com/refraction-networking/utls"
	"strings"
)

var (
	// This should be kept in sync with what is available in utls.
	clientHelloIDMap = map[string]*utls.ClientHelloID{
		"hellogolang":           nil, // Don't bother with utls.
		"hellorandomized":       &utls.HelloRandomized,
		"hellorandomizedalpn":   &utls.HelloRandomizedALPN,
		"hellorandomizednoalpn": &utls.HelloRandomizedNoALPN,
		"hellofirefox_auto":     &utls.HelloFirefox_Auto,
		"hellofirefox_55":       &utls.HelloFirefox_55,
		"hellofirefox_56":       &utls.HelloFirefox_56,
		"hellofirefox_63":       &utls.HelloFirefox_63,
		"hellofirefox_65":       &utls.HelloFirefox_65,
		"hellofirefox_99":       &utls.HelloFirefox_99,
		"hellofirefox_102":      &utls.HelloFirefox_102,
		"hellofirefox_105":      &utls.HelloFirefox_105,
		"hellochrome_auto":      &utls.HelloChrome_Auto,
		"hellochrome_58":        &utls.HelloChrome_58,
		"hellochrome_62":        &utls.HelloChrome_62,
		"hellochrome_70":        &utls.HelloChrome_70,
		"hellochrome_72":        &utls.HelloChrome_72,
		"hellochrome_83":        &utls.HelloChrome_83,
		"hellochrome_87":        &utls.HelloChrome_87,
		"hellochrome_96":        &utls.HelloChrome_96,
		"hellochrome_100":       &utls.HelloChrome_100,
		"hellochrome_102":       &utls.HelloChrome_102,
		"helloios_auto":         &utls.HelloIOS_Auto,
		"helloios_11_1":         &utls.HelloIOS_11_1,
		"helloios_12_1":         &utls.HelloIOS_12_1,
		"helloios_13":           &utls.HelloIOS_13,
		"helloios_14":           &utls.HelloIOS_14,
		"helloandroid_11":       &utls.HelloAndroid_11_OkHttp,
		"helloedge_auto":        &utls.HelloEdge_Auto,
		"helloedge_85":          &utls.HelloEdge_85,
		"helloedge_106":         &utls.HelloEdge_106,
		"hellosafari_auto":      &utls.HelloSafari_Auto,
		"hellosafari_16_0":      &utls.HelloSafari_16_0,
		"hello360_auto":         &utls.Hello360_Auto,
		"hello360_7_5":          &utls.Hello360_7_5,
		"hello360_11_0":         &utls.Hello360_11_0,
		"helloqq_auto":          &utls.HelloQQ_Auto,
		"helloqq_11_1":          &utls.HelloQQ_11_1,
	}
	defaultClientHello = &utls.HelloChrome_Auto
)

func ParseClientHelloID(s string) (*utls.ClientHelloID, error) {
	s = strings.ToLower(s)
	switch s {
	case "none":
		return nil, nil
	case "":
		return defaultClientHello, nil
	default:
		if ret := clientHelloIDMap[s]; ret != nil {
			return ret, nil
		}
	}
	return nil, fmt.Errorf("invalid ClientHelloID: '%v'", s)
}
