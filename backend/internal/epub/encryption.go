package epub

import (
	"context"
	"encoding/xml"
	"fmt"
)

// Obfuscated fonts are harmless only because publisher fonts are not rendered
// in this profile. Never exempt XHTML/images merely because their algorithm is
// normally used for fonts, and never return obfuscated bytes as public assets.
func checkEncryption(ctx context.Context, a *archive, items map[string]Item) error {
	const name = "META-INF/encryption.xml"
	if a.files[name] == nil {
		return ctx.Err()
	}
	data, err := a.readMetadata(ctx, name)
	if err != nil {
		return err
	}
	var doc struct {
		XMLName xml.Name `xml:"urn:oasis:names:tc:opendocument:xmlns:container encryption"`
		Entries []struct {
			Method struct {
				Algorithm string `xml:"Algorithm,attr"`
			} `xml:"http://www.w3.org/2001/04/xmlenc# EncryptionMethod"`
			Cipher struct {
				Reference struct {
					URI string `xml:"URI,attr"`
				} `xml:"http://www.w3.org/2001/04/xmlenc# CipherReference"`
			} `xml:"http://www.w3.org/2001/04/xmlenc# CipherData"`
		} `xml:"http://www.w3.org/2001/04/xmlenc# EncryptedData"`
	}
	if err := decodeXML(ctx, data, &doc); err != nil {
		return err
	}
	if len(doc.Entries) == 0 {
		return ErrPackage
	}
	for _, entry := range doc.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		ref, err := resolveReference("", entry.Cipher.Reference.URI)
		if err != nil || ref.Fragment != "" {
			return ErrPackage
		}
		item, exists := items[ref.Path]
		if !exists {
			return ErrPackage
		}
		algorithm := entry.Method.Algorithm
		obfuscation := algorithm == "http://www.idpf.org/2008/embedding" || algorithm == "http://ns.adobe.com/pdf/enc#RC"
		if !obfuscation || !isFontMediaType(item.MediaType) {
			return fmt.Errorf("%w: %w", ErrUnsupported, ErrEncrypted)
		}
	}
	return nil
}

func isFontMediaType(mediaType string) bool {
	switch mediaType {
	case "font/otf", "font/ttf", "font/woff", "font/woff2", "font/sfnt",
		"application/font-sfnt", "application/font-woff", "application/vnd.ms-opentype", "application/x-font-ttf", "application/x-font-opentype":
		return true
	default:
		return false
	}
}
