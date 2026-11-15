package web

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

func registeredUserPhoneWorkbook(phones []string, exportedAt time.Time) ([]byte, error) {
	const maxRows = 1_048_576
	if len(phones) >= maxRows {
		return nil, errors.New("phone export exceeds the XLSX worksheet row limit")
	}

	var result bytes.Buffer
	workbook := zip.NewWriter(&result)
	for _, part := range []struct{ name, content string }{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`},
		{"xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="sheet1" sheetId="1" r:id="rId1"/></sheets>
</workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`},
	} {
		entry, err := workbook.Create(part.name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(entry, part.content); err != nil {
			return nil, err
		}
	}

	sheet, err := workbook.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(sheet, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<dimension ref="A1:B%d"/><cols><col min="1" max="2" width="22" customWidth="1"/></cols><sheetData>`, len(phones)+1); err != nil {
		return nil, err
	}

	encoder := xml.NewEncoder(sheet)
	var row struct {
		XMLName xml.Name `xml:"row"`
		Number  int      `xml:"r,attr"`
		Cells   [2]struct {
			Reference string `xml:"r,attr"`
			Type      string `xml:"t,attr"`
			Value     string `xml:"is>t"`
		} `xml:"c"`
	}
	// Use text cells so Excel preserves prefixes and never treats a value as a formula.
	for i := range row.Cells {
		row.Cells[i].Type = "inlineStr"
	}
	timestamp := exportedAt.Format("2006-01-02 15:04:05")
	for i := 0; i <= len(phones); i++ {
		row.Number = i + 1
		rowNumber := strconv.Itoa(row.Number)
		row.Cells[0].Reference = "A" + rowNumber
		row.Cells[1].Reference = "B" + rowNumber
		if i == 0 {
			row.Cells[0].Value = "phone"
			row.Cells[1].Value = "time"
		} else {
			row.Cells[0].Value = phones[i-1]
			row.Cells[1].Value = timestamp
		}
		if err := encoder.Encode(row); err != nil {
			return nil, err
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}

	if _, err := io.WriteString(sheet, "</sheetData></worksheet>"); err != nil {
		return nil, err
	}
	if err := workbook.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}
