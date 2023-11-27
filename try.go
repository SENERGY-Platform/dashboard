package main 

import (
	"strings"
	"reflect"
	"fmt"
	"encoding/json"
)

type A struct {
	A *struct {
		B int
	}
}

func main() {

	var a interface{}

	err := json.Unmarshal([]byte("{\"a\": {\"b\": {\"c\": 2, \"d\": 5}}}"), &a)
	if err != nil {
		panic(err)
	}

	propertyToChange := "a.b.c"
	propertyPath := strings.Split(propertyToChange, ".")
	i_last_prop := len(propertyPath) - 1
	newValue := 66 

	var currentValue interface{}
	currentValue = a
	for i, property := range propertyPath {
		val := reflect.ValueOf(currentValue)
		fmt.Println("VAL: ", val)
		fmt.Println("TYPE: ", val.Kind())

		if val.Kind() == reflect.Map {
			if i == i_last_prop {
				val.SetMapIndex(reflect.ValueOf(property), reflect.ValueOf(newValue))
				break
			}

			temp := val.MapIndex(reflect.ValueOf(property)) // why interface?
			if !temp.IsValid() {
				return 
			}
			currentValue = temp.Interface()
		} 
	} 

	fmt.Print(a)
}
