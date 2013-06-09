package v0

import (
  "fmt"
  "os"
  "strings"
  "io"
  "strconv"
  "bytes"
  "path/filepath"

  "github.com/ernestokarim/cb/config"
  "github.com/kylelemons/go-gypsy/yaml"
  "github.com/ernestokarim/cb/registry"
)

func init() {
  registry.NewUserTask("validator", 0, validator)
}

type Field struct {
  Key, Kind, Store, Condition string
  Validators []*Validator
  Fields []*Field
}

type Validator struct {
  Name, Value string
  Uses []string
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

  namespace := data.GetDefault("namespace", "")
  if len(namespace) != 0 {
    namespace = `\` + namespace
  }
  name := data.GetRequired("name")
  root := data.GetDefault("root", "Object")
  if root != "Object" && root != "Array" {
    return fmt.Errorf("invalid root type, only 'object' and 'array' are accepted")
  }
  fields := parseFields(data, "fields")
  if err := generator(filename, name, namespace, root, fields); err != nil {
    return fmt.Errorf("generator error: %s", err)
  }

  return nil
}

// ==================================================================

func parseFields(data *config.Config, spec string) []*Field {
  fields := []*Field{}

  size := data.CountRequired("%s", spec)
  for i := 0; i < size; i++ {
    field := &Field{
      Key: data.GetDefault("%s[%d].key", "", spec, i),
      Kind: data.GetRequired("%s[%d].kind", spec, i),
      Store: data.GetDefault("%s[%d].store", "", spec, i),
      Condition: data.GetDefault("%s[%d].condition", "", spec, i),
      Validators: make([]*Validator, 0),
    }

    if field.Kind == "Array" || field.Kind == "Object" || field.Kind == "Conditional" {
      newSpec := fmt.Sprintf("%s[%d].fields", spec, i)
      field.Fields = parseFields(data, newSpec)
    }

    validatorsSize := data.CountDefault("%s[%d].validators", spec, i)
    for j := 0; j < validatorsSize; j++ {
      v := &Validator{
        Name: data.GetRequired("%s[%d].validators[%d].name", spec, i, j),
        Value: data.GetDefault("%s[%d].validators[%d].value", "", spec, i, j),
      }

      usesSize := data.CountDefault("%s[%d].validators[%d].use", spec, i, j)
      for k := 0; k < usesSize; k++ {
        value := data.GetDefault("%s[%d].validators[%d].use[%d]", "", spec, i, j, k)
        v.Uses = append(v.Uses, value)
      }

      field.Validators = append(field.Validators, v)
    }

    fields = append(fields, field)
  }

  return fields
}

// ==================================================================

type emitter struct {
  f io.Writer
  indentation int
  uses []string
  id int
}

func (e *emitter) indent() {
  e.indentation += 2
}

func (e *emitter) unindent() {
  e.indentation -= 2
}

func (e *emitter) addUse(use string) {
  for _, u := range e.uses {
    if u == use {
      return
    }
  }
  e.uses = append(e.uses, use)
}

func (e *emitter) emitf(format string, a ...interface{}) {
  for i := 0; i < e.indentation; i++ {
    fmt.Fprint(e.f, " ")
  }
  fmt.Fprintf(e.f, fmt.Sprintf(format, a...))
  fmt.Fprintln(e.f)
}

func (e *emitter) arrayId() int {
  id := e.id
  e.id++
  return id
}

// ==================================================================

func generator(filename, name, namespace, root string, fields []*Field) error {
  f, err := os.Create(filepath.Join(filepath.Dir(filename), name + ".php"))
  if err != nil {
    return fmt.Errorf("cannot create dest file: %s", err)
  }
  defer f.Close()

  buf := bytes.NewBuffer(nil)
  e := &emitter{f: buf, indentation: 4}

  if root == "Object" {
    if err := generateObject(e, "data", "valid", fields); err != nil {
      return fmt.Errorf("generate object fields failed: %s", err)
    }
  } else if root == "Array" {
    if err := generateArray(e, "data", "valid", fields); err != nil {
      return fmt.Errorf("generate array fields failed: %s", err)
    }
  }

  var uses string
  for _, use := range e.uses {
    uses += "\nuse " + use + ";"
  }

  fmt.Fprintf(f, `<?php namespace Validators%s;
// AUTOGENERATED BY cb FROM %s, PLEASE, DON'T MODIFY IT

use App;
use Input;
use Log;
%s

class %s {

  public static function validateJson() {
    return self::validateData(Input::json()->all());
  }

  public static function error($data, $msg) {
    $bt = debug_backtrace();
    $caller = array_shift($bt);
    Log::error($msg);
    Log::debug($caller['file'] . '::' . $caller['line']);
    Log::debug(var_export($data, TRUE));
    App::abort(403);
  }

  public static function validateData($data) {
    $valid = array();
    $store = array();

    if (!is_array($data)) {
      self::error($data, 'root is not an array');
    }

%s
    if (!$valid) {
      Log::warning('$valid is not evaluated to true');
      Log::debug(var_export($data, TRUE));
      Log::debug(var_export($valid, TRUE));
      App::abort(403);
    }
    return $valid;
  }

}

`, namespace, filename, uses, name, buf.String())
  return nil
}

func generateObject(e *emitter, varname, result string, fields []*Field) error {
  for _, f := range fields {
    f.Key = "'" + f.Key + "'"

    if f.Kind != "Conditional" {
      e.emitf(`if (!isset($%s[%s])) {`, varname, f.Key)
      e.emitf(`  $%s[%s] = null;`, varname, f.Key);
      e.emitf(`}`)
    }

    if err := generateField(e, f, varname, result); err != nil {
      return fmt.Errorf("generate field failed: %s", err)
    }
    if f.Kind != "Conditional" {
      if err := generateValidators(e, f); err != nil {
        return fmt.Errorf("generate validators failed: %s", err)
      }

      if f.Kind != "Array" && f.Kind != "Object" {
        e.emitf(`$%s[%s] = $value;`, result, f.Key)
      }
    }
    e.emitf("")
  }
  return nil
}

func generateArray(e *emitter, varname, result string, fields []*Field) error {
  id := e.arrayId()
  e.emitf("$size%d = count($%s);", id, varname)
  e.emitf("for ($i%d = 0; $i%d < $size%d; $i%d++) {", id, id, id, id)
  e.indent()

  for _, f := range fields {
    f.Key = fmt.Sprintf("$i%d", id)

    if err := generateField(e, f, varname, result); err != nil {
      return fmt.Errorf("generate field failed: %s", err)
    }
    if f.Kind != "Conditional" {
      if err := generateValidators(e, f); err != nil {
        return fmt.Errorf("generate validators failed: %s", err)
      }

      if f.Kind != "Array" && f.Kind != "Object" {
        e.emitf(`$%s[%s] = $value;`, result, f.Key)
      }
    }
    e.emitf("")
  }

  e.unindent()
  e.emitf("}")
  return nil
}

func generateField(e *emitter, f *Field, varname, result string) error {
  switch f.Kind {
  case "String":
    e.emitf(`$value = $%s[%s];`, varname, f.Key)
    e.emitf(`if ($value === null) {`)
    e.emitf(`  $value = '';`)
    e.emitf(`}`)
    e.emitf(`if (is_int($value)) {`)
    e.emitf(`  $value = strval($value);`)
    e.emitf(`}`)
    e.emitf(`if (!is_string($value)) {`)
    e.emitf(`  self::error($data, 'key ' . %s . ' is not a string');`, f.Key);
    e.emitf(`}`)

  case "Integer":
    e.emitf(`$value = $%s[%s];`, varname, f.Key)
    e.emitf(`if ($value === null) {`)
    e.emitf(`  $value = 0;`)
    e.emitf(`}`)
    e.emitf(`if (is_string($value)) {`)
    e.emitf(`  if (!ctype_digit($value)) {`)
    e.emitf(`    self::error($data, 'key ' . %s . ' is not a valid int');`, f.Key);
    e.emitf(`  }`)
    e.emitf(`  $value = intval($value);`)
    e.emitf(`}`)
    e.emitf(`if (!is_int($value)) {`)
    e.emitf(`  self::error($data, 'key ' . %s . ' is not an int');`, f.Key);
    e.emitf(`}`)

  case "Boolean":
    e.emitf(`$value = $%s[%s];`, varname, f.Key)
    e.emitf(`if (is_string($value)) {`)
    e.emitf(`  if ($value === 'true' || $value === '1' || $value === 'on') {`)
    e.emitf(`    $value = true;`);
    e.emitf(`  }`)
    e.emitf(`  if ($value === 'false' || $value === '0' || $value === 'off') {`)
    e.emitf(`    $value = false;`);
    e.emitf(`  }`)
    e.emitf(`}`)
    e.emitf(`if (!is_bool($value)) {`)
    e.emitf(`  self::error($data, 'key ' . %s . ' is not a boolean');`, f.Key);
    e.emitf(`}`)

  case "Object":
    e.emitf(`$value = $%s[%s];`, varname, f.Key)
    e.emitf(`if (!is_array($value)) {`)
    e.emitf(`  self::error($data, 'key ' . %s . ' is not an object');`, f.Key);
    e.emitf(`}`)
    e.emitf(`$%s[%s] = array();`, result, f.Key)
    e.emitf("")

    name := fmt.Sprintf("%s[%s]", varname, f.Key)
    res := fmt.Sprintf("%s[%s]", result, f.Key)
    if err := generateObject(e, name, res, f.Fields); err != nil {
      return fmt.Errorf("generate object failed: %s", err)
    }

  case "Array":
    e.emitf(`$value = $%s[%s];`, varname, f.Key)
    e.emitf(`if (!is_array($value)) {`)
    e.emitf(`  self::error($data, 'key ' . %s . ' is not an array');`, f.Key);
    e.emitf(`}`)
    e.emitf(`$%s[%s] = array();`, result, f.Key)
    e.emitf("")

    name := fmt.Sprintf("%s[%s]", varname, f.Key)
    res := fmt.Sprintf("%s[%s]", result, f.Key)
    if err := generateArray(e, name, res, f.Fields); err != nil {
      return fmt.Errorf("generate array failed: %s", err)
    }

  case "Conditional":
    if len(f.Condition) == 0 {
      return fmt.Errorf("conditional node needs a condition")
    }

    e.emitf(`if (%s) {`, f.Condition)
    e.indent()

    if err := generateObject(e, varname, result, f.Fields); err != nil {
      return fmt.Errorf("generate object failed: %s", err)
    }

    e.unindent()
    e.emitf(`}`)

  default:
    return fmt.Errorf("`%s` is not a valid field kind", f.Kind)
  }

  if f.Store != "" {
    e.emitf(`$store['%s'] = $value;`, f.Store)
  }

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
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the minlength validation');`, f.Key);
      e.emitf(`}`)

    case "Email":
      e.emitf(`if (filter_var($value, FILTER_VALIDATE_EMAIL) === false) {`)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the email validation');`, f.Key);
      e.emitf(`}`)

    case "DbPresent":
      if v.Value == "" {
        return fmt.Errorf("DbPresent filter needs an entity name as value")
      }
      e.addUse(v.Value)
      e.emitf(`if (!%s::find($value)) {`, v.Value)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the dbpresent validation');`, f.Key);
      e.emitf(`}`)

    case "DbPresentNullable":
      if v.Value == "" {
        return fmt.Errorf("DbPresentNullable filter needs an entity name as value")
      }
      e.addUse(v.Value)
      e.emitf(`if ($value !== '' && $value !== '0' && !%s::find($value)) {`, v.Value)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the dbpresent validation');`, f.Key);
      e.emitf(`}`)

    case "In":
      if v.Value == "" {
        return fmt.Errorf("In filter needs a list of items as value")
      }
      val := strings.Join(strings.Split(v.Value, ","), `', '`)
      e.emitf(`if (!in_array($value, array('%s'), TRUE)) {`, val)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the in validation');`, f.Key);
      e.emitf(`}`)

    case "Date":
      e.emitf(`$str = explode('/', $value);`)
      e.emitf(`if (count($str) !== 3 || !checkdate($str[1], $str[0], $str[2])) {`)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the date validation');`, f.Key);
      e.emitf(`}`)

    case "OptionalDate":
      e.emitf(`$str = explode('/', $value);`)
      e.emitf(`if (count($str) === 3 && !checkdate($str[1], $str[0], $str[2])) {`)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the date validation');`, f.Key);
      e.emitf(`}`)

    case "Before":
      if v.Value == "" {
        return fmt.Errorf("Before filter needs a date as value")
      }
      e.addUse("DateTime")
      e.emitf(`$str = explode('/', $value);`)
      e.emitf(`if (count($str) === 3 &&
          DateTime::createFromFormat('d/m/Y', $value) >= new DateTime('%s')) {`, v.Value)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the before validation');`, f.Key);
      e.emitf(`}`)

    case "Custom":
      for _, u := range v.Uses {
        e.addUse(u)
      }
      e.emitf(`if (%s) {`, v.Value)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the custom validation');`, f.Key);
      e.emitf(`}`)

    case "Positive":
      e.emitf(`if ($value < 0) {`)
      e.emitf(`  self::error($data, 'key ' . %s . ' breaks the positive validation');`, f.Key);
      e.emitf(`}`)

    default:
      return fmt.Errorf("`%s` is not a valid validator name", v.Name)
    }
  }
  return nil
}
