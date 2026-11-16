package icontheme

import "encoding/base64"

// base64Encode encodes data as standard base64.
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
