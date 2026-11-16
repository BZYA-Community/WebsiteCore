package web

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io/fs"
	"strconv"
	"testing"
	"time"
)

func TestRegisteredUserPhoneWorkbook(t *testing.T) {
	exportedAt := time.Date(2026, time.November, 15, 1, 2, 3, 456, time.FixedZone("UTC+8", 8*60*60))
	for _, tt := range []struct {
		name   string
		phones []string
	}{
		{name: "empty"},
		{name: "text values", phones: []string{"01234567890", "+8613800000000", "=A1&<A2"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := registeredUserPhoneWorkbook(tt.phones, exportedAt)
			if err != nil {
				t.Fatal(err)
			}
			archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			var workbook struct {
				Sheets []struct {
					Name string `xml:"name,attr"`
				} `xml:"sheets>sheet"`
			}
			workbookXML, err := fs.ReadFile(archive, "xl/workbook.xml")
			if err != nil {
				t.Fatal(err)
			}
			if err := xml.Unmarshal(workbookXML, &workbook); err != nil {
				t.Fatal(err)
			}
			if len(workbook.Sheets) != 1 || workbook.Sheets[0].Name != "sheet1" {
				t.Fatalf("unexpected worksheets: %+v", workbook.Sheets)
			}

			var worksheet struct {
				Rows []struct {
					Cells []struct {
						Reference string `xml:"r,attr"`
						Type      string `xml:"t,attr"`
						Value     string `xml:"is>t"`
					} `xml:"c"`
				} `xml:"sheetData>row"`
			}
			sheetXML, err := fs.ReadFile(archive, "xl/worksheets/sheet1.xml")
			if err != nil {
				t.Fatal(err)
			}
			if err := xml.Unmarshal(sheetXML, &worksheet); err != nil {
				t.Fatal(err)
			}
			if len(worksheet.Rows) != 1+len(tt.phones) {
				t.Fatalf("worksheet row count = %d, want %d", len(worksheet.Rows), 1+len(tt.phones))
			}
			for i, row := range worksheet.Rows {
				if len(row.Cells) != 2 {
					t.Fatalf("expected two text cells in row: %+v", row)
				}
				phone, timestamp := "phone", "time"
				if i > 0 {
					phone, timestamp = tt.phones[i-1], "2026-11-15 01:02:03"
				}
				for col, value := range []string{phone, timestamp} {
					cell := row.Cells[col]
					reference := string("AB"[col]) + strconv.Itoa(i+1)
					if cell.Reference != reference || cell.Type != "inlineStr" || cell.Value != value {
						t.Errorf("expected %s to contain %q as text, got %+v", reference, value, cell)
					}
				}
			}
		})
	}
}
