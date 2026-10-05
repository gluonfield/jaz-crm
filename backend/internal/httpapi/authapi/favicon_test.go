package authapi_test

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"image/png"
	"io"
	"net/http"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	signin "github.com/gluonfield/jaz-tasks/auth"
)

func TestPublicFavicons(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	for _, format := range []string{"svg", "ico"} {
		res, err := http.Get(s.url + "/favicon." + format)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("favicon %s: %d %v", format, res.StatusCode, err)
		}
		if format == "svg" {
			var root struct{ XMLName xml.Name }
			if err := xml.Unmarshal(body, &root); err != nil || root.XMLName.Local != "svg" || res.Header.Get("Content-Type") != "image/svg+xml" {
				t.Fatalf("invalid SVG favicon: %v", err)
			}
			continue
		}
		if len(body) < 22 || binary.LittleEndian.Uint16(body[2:4]) != 1 || binary.LittleEndian.Uint16(body[4:6]) != 1 || res.Header.Get("Content-Type") != "image/x-icon" {
			t.Fatal("invalid ICO response")
		}
		offset := binary.LittleEndian.Uint32(body[18:22])
		if offset >= uint32(len(body)) {
			t.Fatal("invalid ICO image offset")
		}
		image, err := png.Decode(bytes.NewReader(body[offset:]))
		if err != nil || image.Bounds().Dx() != int(body[6]) || image.Bounds().Dy() != int(body[7]) {
			t.Fatalf("invalid ICO image: %v", err)
		}
	}
}
