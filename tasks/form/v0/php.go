package v0

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/ernestokarim/cb/config"
	"github.com/ernestokarim/cb/registry"
)

func init() {
	registry.NewUserTask("form:php", 0, form_php)
}

var (
	phpTemplate = `<?php
// AUTOGENERATED BY cb FROM {{ .Filename }}, PLEASE, DON'T MODIFY IT

class {{ .Classname }} {

  public static function validate() {
    $data = Input::json(true);
    $rules = array(
      {{ range .Rules }}'{{ .Name }}' => '{{ .Validators }}',
      {{ end }}
    );

    $validation = Validator::make($data, $rules);
    if ($validation->fails())
      return null;

    return $data;
  }

}
`

	phpNameTable = map[string]string{
		"required":   "required",
		"minlength":  "min",
		"email":      "email",
		"dateBefore": "before",
		"boolean":    "in",
	}
)

type PhpData struct {
	Filename  string
	Classname string
	Rules     []*Rule
}

type Rule struct {
	Name, Validators string
}

func form_php(c *config.Config, q *registry.Queue) error {
	data, filename, err := loadData()
	if err != nil {
		return fmt.Errorf("read data failed: %s", err)
	}

	tdata := &PhpData{
		Filename: filename,
		Rules:    make([]*Rule, 0),
	}

	tdata.Classname, err = data.Get("classname")
	if err != nil {
		return fmt.Errorf("get config failed: %s", err)
	}

	size, err := data.Count("fields")
	if err != nil {
		return fmt.Errorf("count config failed: %s", err)
	}
	for i := 0; i < size; i++ {
		name, err := data.GetStringf("fields[%d].name", i)
		if err != nil {
			return fmt.Errorf("get config failed: %s", err)
		}

		validatorsSize, err := data.Countf("fields[%d].validators", i)
		if err != nil {
			return fmt.Errorf("count config failed for %s: %s", name, err)
		}
		validators := []string{}
		for j := 0; j < validatorsSize; j++ {
			vname, err := data.GetStringf("fields[%d].validators[%d].name", i, j)
			if err != nil {
				return fmt.Errorf("get validator name failed: %s", err)
			}
			vvalue, err := data.GetStringf("fields[%d].validators[%d].value", i, j)
			if err != nil && !config.IsNotFound(err) {
				return fmt.Errorf("get validator value failed: %s", err)
			}
			if vname == "boolean" {
				vvalue = "true,false"
			}

			vname = phpNameTable[vname]

			val := fmt.Sprintf("%s", vname)
			if len(vvalue) > 0 {
				val = fmt.Sprintf("%s:%s", vname, vvalue)
			}
			validators = append(validators, val)
		}

		fieldType, err := data.GetStringf("fields[%d].type", i)
		if err != nil {
			return fmt.Errorf("get field type failed: %s", err)
		}
		if fieldType == "radiobtn" {
			values := extractRadioBtnValues(data, i)
			keys := []string{}
			for key := range values {
				keys = append(keys, key)
			}
			validators = append(validators, "in:"+strings.Join(keys, ","))
		}

		tdata.Rules = append(tdata.Rules, &Rule{
			Name:       name,
			Validators: strings.Join(validators, "|"),
		})
	}

	if err := build_php(tdata); err != nil {
		return fmt.Errorf("build form failed: %s", err)
	}

	return nil
}

func build_php(data *PhpData) error {
	t, err := template.New("php").Parse(phpTemplate)
	if err != nil {
		return fmt.Errorf("parse template failed: %s", err)
	}

	if err := t.Execute(os.Stdout, data); err != nil {
		return fmt.Errorf("execute template failed: %s", err)
	}

	return nil
}
