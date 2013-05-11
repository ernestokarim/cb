package v0

type Validator struct {
  Attrs   map[string]string
  Message string
  Error   string
}

func initValidator(name, value, msg string) *Validator {
  m := map[string]func(name, value, msg string) *Validator{
    "required": required,
    "minlength": minLength,
    "maxlength": maxLength,
    "email": email,
  }
  return m[name](name, value, msg)
}

func required(name, value, msg string) *Validator {
  return &Validator{
    Attrs:   map[string]string{"required": ""},
    Message: msg,
    Error:   "required",
  }
}

func minLength(name, value, msg string) *Validator {
  return &Validator{
    Attrs:   map[string]string{"ng-minlength": value},
    Message: msg,
    Error:   "minlength",
  }
}

func maxLength(name, value, msg string) *Validator {
  return &Validator{
    Attrs:   map[string]string{"ng-maxlength": value},
    Message: msg,
    Error:   "maxlength",
  }
}

func email(name, value, msg string) *Validator {
  return &Validator{
    Attrs:   map[string]string{},
    Message: msg,
    Error:   "email",
  }
}
