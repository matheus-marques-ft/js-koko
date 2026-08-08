package common

import (
	"fmt"
	"testing"
)

func TestNewTable_CalculateColumnsSize(t *testing.T) {
	table := WrapperTable{
		Fields: []string{"ID", "Hostname", "IP", "SystemUser", "Comment"},
		Data: []map[string]string{
			{"ID": "1", "Hostname": "asdfasdf", "IP": "192.168.1.1", "SystemUser": "123", "Comment": "Hello"},
			{"ID": "2", "Hostname": "bbb", "IP": "255.255.255.255", "SystemUser": "o", "Comment": ""},
			{"ID": "3", "Hostname": "3", "IP": "1.1.1.1", "SystemUser": "", "Comment": "aaaa"},
			{"ID": "3", "Hostname": "22323", "IP": "1.1.2.1", "SystemUser": "", "Comment": ""},
			{"ID": "2", "Hostname": "22323", "IP": "192.168.1.1", "SystemUser": "", "Comment": ""},
		},
		FieldsSize: map[string][3]int{
			"ID":         {0, 0, 5},
			"Hostname":   {0, 8, 25},
			"IP":         {15, 0, 0},
			"SystemUser": {0, 12, 20},
			"Comment":    {0, 0, 0},
		},
		TotalSize: 140,
	}
	table.Initial()

	data := table.Display()
	fmt.Println(data)
	fmt.Println(table.fieldsSize)
}


func TestGetCorrectString(t *testing.T) {
	foo := "主2erert机名"
	a:=GetValidString(foo,2,false)
	t.Log(a == "2erert机名")
}