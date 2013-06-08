package v0

import (
  "fmt"
  "os"
  "strings"
  "io"
  "strconv"

  "github.com/ernestokarim/cb/config"
  "github.com/kylelemons/go-gypsy/yaml"
  "github.com/ernestokarim/cb/registry"
)

func init() {
  registry.NewUserTask("validator", 0, validator)
}

type Field struct {
  Key, Kind string
  Validators []*Validator
  Fields []*Field
}

type Validator struct {
  Name, Value string
}

func validator(c *config.Config, q *registry.Queue) error {
  filename := q.NextTask()
  if filename == "" {
    return fmt.Errorf("validator filename not passed as an argument")
  }
  q.RemoveNextTask()

  f, err := yaml.ReadFile(filename)
  if err != nil {
    return fmt.Errorf("read validator failed: %s", err)
  }
  data := config.NewConfig(f)

  name := data.GetRequired("name")
  fields := parseFields(data, "fields")
  if err := generator(filename, name, fields); err != nil {
    return fmt.Errorf("generator error: %s", err)
  }

  return nil
}

func parseFields(data *config.Config, spec string) []*Field {
  fields := []*Field{}

  size := data.CountRequired("%s", spec)
  for i := 0; i < size; i++ {
    field := &Field{
      Key: data.GetRequired("%s[%d].key", spec, i),
      Kind: data.GetRequired("%s[%d].kind", spec, i),
      Validators: make([]*Validator, 0),
    }

    if field.Kind == "Array" {
      newSpec := fmt.Sprintf("%s[%d].fields", spec, i)
      field.Fields = parseFields(data, newSpec)
    }

    validatorsSize := data.CountDefault("%s[%d].validators", spec, i)
    for j := 0; j < validatorsSize; j++ {
      field.Validators = append(field.Validators, &Validator{
        Name: data.GetRequired("%s[%d].validators[%d].name", spec, i, j),
        Value: data.GetDefault("%s[%d].validators[%d].value", "", spec, i, j),
      })
    }

    fields = append(fields, field)
  }

  return fields
}

type emitter struct {
  f io.Writer
  indentation int
}

func (e *emitter) indent() {
  e.indentation += 2
}

func (e *emitter) unindent() {
  e.indentation -= 2
}

func (e *emitter) emitf(format string, a ...interface{}) {
  for i := 0; i < e.indentation; i++ {
    fmt.Fprint(e.f, " ")
  }
  fmt.Fprintf(e.f, fmt.Sprintf(format, a...))
  fmt.Fprintln(e.f)
}

func generator(filename, name string, fields []*Field) error {
  f, err := os.Create(name + ".php")
  if err != nil {
    return fmt.Errorf("cannot create dest file: %s", err)
  }
  defer f.Close()

  fmt.Fprintf(f, `<?php namespace Validators;
// AUTOGENERATED BY cb FROM %s, PLEASE, DON'T MODIFY IT

use Input;
use Log;

class %s {

  public static function validateJson() {
    return self::validate(Input::json()->all());
  }

  public static function error($data, $msg) {
    $bt = debug_backtrace();
    $caller = array_shift($bt);
    Log::error($msg);
    Log::debug($caller['file'] . '::' . $caller['line']);
    Log::debug(var_export($data, TRUE));
    return false;
  }

  public static function validate($data) {
    $valid = array();

    if (!is_array($data)) {
      return self::error($data, 'root is not an array');
    }

`, filename, name)

  e := &emitter{f: f, indentation: 2}
  if err := generateFields(e, "data", "valid", fields); err != nil {
    return fmt.Errorf("generate fields failed: %s", err)
  }

  fmt.Fprintf(f, "    return $valid;\n  }\n\n}")
  return nil
}

func generateFields(e *emitter, varname, result string, fields []*Field) error {
  e.indent()

  for _, f := range fields {
    e.emitf(`if (!isset($%s['%s'])) {`, varname, f.Key)
    e.emitf(`  return self::error($data, 'key "%s" not set in data');`, f.Key);
    e.emitf(`}`)

    e.emitf(`$value = $%s['%s'];`, varname, f.Key)
    switch f.Kind {
    case "String":
      e.emitf(`if (is_int($value)) {`)
      e.emitf(`  $value = strval($value);`)
      e.emitf(`}`)
      e.emitf(`if (!is_string($value)) {`)
      e.emitf(`  return self::error($data, 'key "%s" is not a string');`, f.Key);
      e.emitf(`}`)

    case "Integer":
      e.emitf(`if (is_string($value)) {`)
      e.emitf(`  if (!ctype_digit($value)) {`)
      e.emitf(`    return self::error($data, 'key "%s" is not a valid int');`, f.Key);
      e.emitf(`  }`)
      e.emitf(`  $value = intval($value);`)
      e.emitf(`}`)
      e.emitf(`if (!is_int($value)) {`)
      e.emitf(`  return self::error($data, 'key "%s" is not an int');`, f.Key);
      e.emitf(`}`)

    case "Boolean":
      e.emitf(`if (!is_bool($value)) {`)
      e.emitf(`  return self::error($data, 'key "%s" is not a boolean');`, f.Key);
      e.emitf(`}`)

    default:
      return fmt.Errorf("`%s` is not a valid field kind", f.Kind)
    }

    if err := generateValidators(e, f); err != nil {
      return fmt.Errorf("generate validators failed: %s", err)
    }
    e.emitf(`$%s['%s'] = $value;`, result, f.Key)
    e.emitf("")
  }

  e.unindent()
  return nil
}

func generateValidators(e *emitter, f *Field) error {
  for _, v := range f.Validators {
    switch v.Name {
    case "MinLength":
      val, err := strconv.ParseInt(v.Value, 10, 64)
      if err != nil {
        return fmt.Errorf("cannot parse minlength number: %s", err)
      }

      e.emitf(`if (!strlen($value) < %d) {`, val)
      e.emitf(`  return self::error($data, 'key "%s" breaks the minlength validation');`, f.Key);
      e.emitf(`}`)

    case "Email":
      e.emitf(`if (filter_var($value, FILTER_VALIDATE_EMAIL) === false) {`)
      e.emitf(`  return self::error($data, 'key "%s" breaks the email validation');`, f.Key);
      e.emitf(`}`)

    case "DbPresent":
      if v.Value == "" {
        return fmt.Errorf("DbPresent filter needs an entity name as value")
      }
      e.emitf(`if (!%s::find($value)) {`, v.Value)
      e.emitf(`  return self::error($data, 'key "%s" breaks the dbpresent validation');`, f.Key);
      e.emitf(`}`)

    case "DbPresentNullable":
      if v.Value == "" {
        return fmt.Errorf("DbPresentNullable filter needs an entity name as value")
      }
      e.emitf(`if ($value !== '' && $value !== '0' && !%s::find($value)) {`, v.Value)
      e.emitf(`  return self::error($data, 'key "%s" breaks the dbpresent validation');`, f.Key);
      e.emitf(`}`)

    case "In":
      if v.Value == "" {
        return fmt.Errorf("In filter needs a list of items as value")
      }
      val := strings.Join(strings.Split(v.Value, ","), `', '`)
      e.emitf(`if (!in_array($value, array('%s'), TRUE)) {`, val)
      e.emitf(`  return self::error($data, 'key "%s" breaks the in validation');`, f.Key);
      e.emitf(`}`)

    case "Date":
      e.emitf(`$str = explode('/', $value);`)
      e.emitf(`if (count($str) !== 3 || !checkdate($str[1], $str[0], $str[2])) {`)
      e.emitf(`  return self::error($data, 'key "%s" breaks the date validation');`, f.Key);
      e.emitf(`}`)

    case "Before":
      if v.Value == "" {
        return fmt.Errorf("Before filter needs a date as value")
      }
      e.emitf(`if (DateTime::createFromFormat('d/m/Y', $value) >= new DateTime('%s')) {`, v.Value)
      e.emitf(`  return self::error($data, 'key "%s" breaks the before validation');`, f.Key);
      e.emitf(`}`)

    default:
      return fmt.Errorf("`%s` is not a valid validator name", v.Name)
    }
  }
  return nil
}
