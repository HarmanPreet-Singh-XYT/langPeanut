package memory

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// TMUnit represents a single translation segment across source and target languages
type TMUnit struct {
	Key        string `json:"key"`
	SourceLang string `json:"source_lang"`
	SourceText string `json:"source_text"`
	TargetLang string `json:"target_lang"`
	TargetText string `json:"target_text"`
}

// ─── TMX (Translation Memory eXchange 1.4b) ───────────────────────────────────

type tmxDoc struct {
	XMLName xml.Name  `xml:"tmx"`
	Version string    `xml:"version,attr"`
	Header  tmxHeader `xml:"header"`
	Body    tmxBody   `xml:"body"`
}

type tmxHeader struct {
	CreationTool        string `xml:"creationtool,attr"`
	CreationToolVersion string `xml:"creationtoolversion,attr"`
	SegType             string `xml:"segtype,attr"`
	O_TF                string `xml:"o-tmf,attr"`
	AdminLang           string `xml:"adminlang,attr"`
	Srclang             string `xml:"srclang,attr"`
	DataType            string `xml:"datatype,attr"`
}

type tmxBody struct {
	TUs []tmxTU `xml:"tu"`
}

type tmxTU struct {
	TUID string   `xml:"tuid,attr,omitempty"`
	TUVs []tmxTUV `xml:"tuv"`
}

type tmxTUV struct {
	Lang    string `xml:"lang,attr,omitempty"`
	XMLLang string `xml:"http://www.w3.org/XML/1998/namespace lang,attr,omitempty"`
	Seg     string `xml:"seg"`
}

func (tuv tmxTUV) GetLang() string {
	if tuv.Lang != "" {
		return tuv.Lang
	}
	return tuv.XMLLang
}

// ExportTMX exports translation units to standard TMX 1.4b XML
func ExportTMX(units []TMUnit, sourceLang string) ([]byte, error) {
	if sourceLang == "" {
		sourceLang = "en"
	}

	doc := tmxDoc{
		Version: "1.4",
		Header: tmxHeader{
			CreationTool:        "langPeanut",
			CreationToolVersion: "1.0.0",
			SegType:             "sentence",
			O_TF:                "unknown",
			AdminLang:           "en-US",
			Srclang:             sourceLang,
			DataType:            "PlainText",
		},
	}

	// Group units by source text and key
	type tuKey struct {
		key        string
		sourceText string
	}
	grouped := make(map[tuKey]map[string]string)

	for _, u := range units {
		k := tuKey{key: u.Key, sourceText: u.SourceText}
		if grouped[k] == nil {
			grouped[k] = make(map[string]string)
		}
		if u.TargetLang != "" && u.TargetText != "" {
			grouped[k][u.TargetLang] = u.TargetText
		}
	}

	for k, targets := range grouped {
		tu := tmxTU{
			TUID: k.key,
			TUVs: []tmxTUV{
				{Lang: sourceLang, Seg: k.sourceText},
			},
		}
		for tgtLang, tgtText := range targets {
			if strings.EqualFold(tgtLang, sourceLang) {
				continue
			}
			tu.TUVs = append(tu.TUVs, tmxTUV{
				Lang: tgtLang,
				Seg:  tgtText,
			})
		}
		doc.Body.TUs = append(doc.Body.TUs, tu)
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

// ImportTMX parses standard TMX XML data into TMUnits
func ImportTMX(data []byte) ([]TMUnit, error) {
	var doc tmxDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid TMX format: %w", err)
	}

	srcLang := doc.Header.Srclang
	if srcLang == "" {
		srcLang = "en"
	}

	var results []TMUnit
	for _, tu := range doc.Body.TUs {
		var srcText string
		tgtMap := make(map[string]string)

		for _, tuv := range tu.TUVs {
			lang := tuv.GetLang()
			if strings.EqualFold(lang, srcLang) || (srcText == "" && len(tu.TUVs) > 1 && lang == doc.Header.Srclang) {
				srcText = tuv.Seg
			} else if lang != "" {
				tgtMap[lang] = tuv.Seg
			}
		}

		// Fallback: first TUV is source if not explicitly matched
		if srcText == "" && len(tu.TUVs) > 0 {
			srcText = tu.TUVs[0].Seg
			for _, tuv := range tu.TUVs[1:] {
				l := tuv.GetLang()
				if l != "" {
					tgtMap[l] = tuv.Seg
				}
			}
		}

		for tgtLang, tgtText := range tgtMap {
			results = append(results, TMUnit{
				Key:        tu.TUID,
				SourceLang: srcLang,
				SourceText: srcText,
				TargetLang: tgtLang,
				TargetText: tgtText,
			})
		}
	}

	return results, nil
}

// ─── XLIFF 1.2 (XML Localization Interchange File Format) ─────────────────────

type xliffDoc struct {
	XMLName xml.Name  `xml:"xliff"`
	Version string    `xml:"version,attr"`
	Files   []xlFile  `xml:"file"`
}

type xlFile struct {
	Original   string  `xml:"original,attr"`
	SourceLang string  `xml:"source-language,attr"`
	TargetLang string  `xml:"target-language,attr,omitempty"`
	Datatype   string  `xml:"datatype,attr"`
	Body       xlBody  `xml:"body"`
}

type xlBody struct {
	TransUnits []xlTransUnit `xml:"trans-unit"`
}

type xlTransUnit struct {
	ID     string `xml:"id,attr"`
	Source string `xml:"source"`
	Target string `xml:"target,omitempty"`
}

// ExportXLIFF exports translation units for a specific language pair to standard XLIFF 1.2
func ExportXLIFF(units []TMUnit, sourceLang, targetLang string) ([]byte, error) {
	if sourceLang == "" {
		sourceLang = "en"
	}

	fileElem := xlFile{
		Original:   "langPeanut-bundle",
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Datatype:   "plaintext",
	}

	for _, u := range units {
		if (u.TargetLang == "" || strings.EqualFold(u.TargetLang, targetLang)) && u.SourceText != "" {
			fileElem.Body.TransUnits = append(fileElem.Body.TransUnits, xlTransUnit{
				ID:     u.Key,
				Source: u.SourceText,
				Target: u.TargetText,
			})
		}
	}

	doc := xliffDoc{
		Version: "1.2",
		Files:   []xlFile{fileElem},
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

// ImportXLIFF parses standard XLIFF XML data into TMUnits
func ImportXLIFF(data []byte) ([]TMUnit, error) {
	var doc xliffDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid XLIFF format: %w", err)
	}

	var results []TMUnit
	for _, f := range doc.Files {
		srcLang := f.SourceLang
		if srcLang == "" {
			srcLang = "en"
		}
		tgtLang := f.TargetLang

		for _, tu := range f.Body.TransUnits {
			results = append(results, TMUnit{
				Key:        tu.ID,
				SourceLang: srcLang,
				SourceText: tu.Source,
				TargetLang: tgtLang,
				TargetText: tu.Target,
			})
		}
	}

	return results, nil
}

// MergeUnitsIntoTM merges parsed TMUnits directly into a TranslationMemory store
func MergeUnitsIntoTM(tm *TranslationMemory, units []TMUnit) int {
	added := 0
	for _, u := range units {
		if u.SourceText != "" && u.TargetLang != "" && u.TargetText != "" {
			tm.Set(u.SourceText, u.TargetLang, u.TargetText)
			added++
		}
	}
	_ = tm.Save()
	return added
}
