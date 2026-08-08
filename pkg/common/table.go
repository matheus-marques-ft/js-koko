package common

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/olekukonko/tablewriter"
)

const (
	TruncSuffix = iota
	TruncPrefix
	TruncMiddle
)

type WrapperTable struct {
	Labels      []string
	Fields      []string
	FieldsSize  map[string][3]int // column width, column min width, column max width
	Data        []map[string]string
	TotalSize   int
	TruncPolicy int

	totalSize   int
	paddingSize int
	bolderSize  int
	fieldsSize  map[string]int // final width after calculation
	Caption     string
}

func (t *WrapperTable) Initial() {
	// if the configured width is smaller than the title's size, override it
	if t.Labels == nil {
		t.Labels = t.Fields
	}
	if len(t.Fields) != len(t.Labels) {
		panic("Labels should be equal with Fields")
	}
	t.paddingSize = 1
	t.bolderSize = 1
	for i, k := range t.Fields {
		titleSize := len(t.Labels[i])
		sizeDefine := t.FieldsSize[k]
		if titleSize > sizeDefine[1] {
			sizeDefine[1] = titleSize
			t.FieldsSize[k] = sizeDefine
		}
	}
}

func (t *WrapperTable) CalculateColumnsSize() {
	t.fieldsSize = make(map[string]int)

	dataColMaxSize := make(map[string]int)
	for _, row := range t.Data {
		for _, colName := range t.Fields {
			if colValue, ok := row[colName]; ok {
				preSize, ok := dataColMaxSize[colName]
				colSize := len(colValue)
				if !ok || colSize > preSize {
					dataColMaxSize[colName] = colSize
				}
			}
		}
	}

	// if the data width is greater than the configured max, use the max
	// if the data max value is less than the min value, use the min value as the column width
	// otherwise the data's max width is used as the column width
	for k, v := range dataColMaxSize {
		size, min, max := t.FieldsSize[k][0], t.FieldsSize[k][1], t.FieldsSize[k][2]
		if size != 0 {
			t.fieldsSize[k] = size
		} else if max != 0 && v > max {
			t.fieldsSize[k] = max
		} else if min != 0 && v < min {
			t.fieldsSize[k] = min
		} else {
			t.fieldsSize[k] = v
		}
	}

	// total column length after calculation
	calSize := 0
	for _, v := range t.fieldsSize {
		calSize += v
	}
	if t.TotalSize == 0 {
		t.totalSize = calSize
		return
	}

	// when calculating the total width, border and padding should be subtracted
	t.totalSize = t.TotalSize - len(t.Fields)*2*t.paddingSize - (len(t.Fields)+1)*t.bolderSize

	// calculate the columns that can be expanded and shrunk
	delta := t.totalSize - calSize
	if delta == 0 {
		return
	}
	var step = 1
	if delta < 0 {
		step = -1
	}
	delta = Abs(delta)
	for delta > 0 {
		canChangeCols := make([]string, 0)
		for k, v := range t.FieldsSize {
			size, min, max := v[0], v[1], v[2]
			switch step {
			// expand
			case 1:
				if size != 0 || (max != 0 && t.fieldsSize[k] >= max) {
					continue
				}
			// shrink
			case -1:
				if size != 0 || t.fieldsSize[k] <= min {
					continue
				}
			}
			canChangeCols = append(canChangeCols, k)
		}
		if len(canChangeCols) == 0 {
			break
		}
		for _, k := range canChangeCols {
			t.fieldsSize[k] += step
			delta--
			if delta == 0 {
				break
			}
		}
	}
}

func (t *WrapperTable) convertDataToSlice() [][]string {
	data := make([][]string, len(t.Data))
	for i, j := range t.Data {
		row := make([]string, len(t.Fields))
		for m, n := range t.Fields {
			columSize := t.fieldsSize[n]
			if len(j[n]) <= columSize {
				row[m] = j[n]
			} else {
				switch t.TruncPolicy {
				case TruncSuffix:
					row[m] = fmt.Sprintf("%s...",
						GetValidString(j[n], columSize-3, true))
				case TruncPrefix:
					row[m] = fmt.Sprintf("...%s",
						GetValidString(j[n], len(j[n])-columSize-3, false))
				case TruncMiddle:
					midValue := (columSize - 3) / 2
					row[m] = fmt.Sprintf("%s...%s", GetValidString(j[n], midValue, true),
						GetValidString(j[n], len(j[n])-midValue, false))
				}
			}

		}
		data[i] = row
	}
	return data
}

func (t *WrapperTable) Display() string {
	t.CalculateColumnsSize()

	tableString := strings.Builder{}
	table := tablewriter.NewWriter(&tableString)
	table.SetBorder(false)
	table.SetHeader(t.Labels)
	colors := make([]tablewriter.Colors, len(t.Fields))
	for i := 0; i < len(t.Fields); i++ {
		colors[i] = tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor}
	}
	table.SetHeaderColor(colors...)
	data := t.convertDataToSlice()
	table.AppendBulk(data)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	for i, j := range t.Fields {
		n := t.fieldsSize[j]
		table.SetColMinWidth(i, n)
	}
	table.SetColWidth(t.totalSize)
	if t.Caption != "" {
		table.SetCaption(true, t.Caption)
	}
	table.Render()
	return tableString.String()
}

func GetValidString(s string, position int, positive bool) string {
	step := 1
	if positive {
		step = -1
	}
	for position >= 0 && position <= len(s) {
		switch positive {
		case true:
			if utf8.ValidString(s[:position]) {
				return s[:position]
			}
		case false:
			if utf8.ValidString(s[position:]) {
				return s[position:]
			}
		}
		position += step
	}
	return ""
}
