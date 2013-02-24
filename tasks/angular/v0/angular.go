package v0

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ernestokarim/cb/config"
	"github.com/ernestokarim/cb/errors"
	"github.com/ernestokarim/cb/registry"
)

var (
	buf         = bufio.NewReader(os.Stdin)
	serviceTmpl = template.Must(template.New("service").Parse(
		`{{ if not .Exists }}'use strict';


var m = angular.module('{{ .Data.Module }}', []);
{{ end }}

m.factory('{{ .Data.Name }}', function() {
  return {};
});
`))
	serviceTestTmpl = template.Must(template.New("serviceTest").Parse(
		`{{ if not .Exists }}'use strict';

{{ else }}
{{ end }}
describe('Service: {{ .Data.Name }}', function() {
  beforeEach(module('{{ .Data.Module }}'));

  var {{ .Data.Name }};
  beforeEach(inject(function($injector) {
    {{ .Data.Name }} = $injector.get('{{ .Data.Name }}');
  }));
});
`))
	controllerTmpl = template.Must(template.New("controller").Parse(
		`{{ if not .Exists }}'use strict';


var m = angular.module('{{ .Data.Module }}', []);
{{ end }}

m.controller('{{ .Data.Name }}', function() {
  // empty
});
`))
	controllerTestTmpl = template.Must(template.New("controllerTest").Parse(
		`{{ if not .Exists }}'use strict';

{{ else }}
{{ end }}
describe('Controller: {{ .Data.Name }}', function() {
  beforeEach(module('{{ .Data.Module }}'));

  var scope;
  beforeEach(inject(function($injector) {
    var $controller = $injector.get('$controller');
    var $rootScope = $injector.get('$rootScope');

    scope = $rootScope.$new();
    $controller('{{ .Data.Name }}', {$scope: scope});
  }));
});
`))
)

func init() {
	registry.NewTask("service", 0, service)
	registry.NewTask("controller", 0, controller)
}

func service(c config.Config, q *registry.Queue) error {
	fmt.Printf(" - Name of the service: ")
	name, err := getLine()
	if err != nil {
		return err
	}

	fmt.Printf(" - Module of the service: ")
	module, err := getLine()
	if err != nil {
		return err
	}

	if err := writeServiceFile(name, module); err != nil {
		return err
	}
	if err := writeServiceTestFile(name, module); err != nil {
		return err
	}

	return nil
}

func controller(c config.Config, q *registry.Queue) error {
	fmt.Printf(" - Name of the controller: ")
	name, err := getLine()
	if err != nil {
		return err
	}

	fmt.Printf(" - Module of the controller: ")
	module, err := getLine()
	if err != nil {
		return err
	}

	fmt.Printf(" - Route of the controller: ")
	route, err := getLine()
	if err != nil {
		return err
	}

	if err := writeControllerFile(name, module, route); err != nil {
		return err
	}
	if err := writeControllerTestFile(name, module, route); err != nil {
		return err
	}

	return nil
}

// ==================================================================

func getLine() (string, error) {
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			return "", errors.New(err)
		}

		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	panic("should not reach here")
}

type FileData struct {
	Data   interface{}
	Exists bool
}

func writeFile(path string, tmpl *template.Template, data interface{}) error {
	exists := true
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			exists = false
		} else {
			return errors.New(err)
		}
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return errors.New(err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, &FileData{data, exists}); err != nil {
		return errors.New(err)
	}

	return nil
}

// ==================================================================

type ServiceData struct {
	Name, Module string
}

func writeServiceFile(name, module string) error {
	parts := strings.Split(module, ".")
	filename := parts[len(parts)-1] + ".js"
	p := filepath.Join("client", "app", "scripts", "services", filename)

	return writeFile(p, serviceTmpl, &ServiceData{name, module})
}

func writeServiceTestFile(name, module string) error {
	parts := strings.Split(module, ".")
	filename := parts[len(parts)-1] + "Spec.js"
	p := filepath.Join("client", "test", "unit", "services", filename)

	return writeFile(p, serviceTestTmpl, &ServiceData{name, module})
}

// ==================================================================

type ControllerData struct {
	Name, Module, Route string
}

func writeControllerFile(name, module, route string) error {
	parts := strings.Split(module, ".")
	filename := parts[len(parts)-1] + ".js"
	p := filepath.Join("client", "app", "scripts", "controllers", filename)

	return writeFile(p, controllerTmpl, &ControllerData{name, module, route})
}

func writeControllerTestFile(name, module, route string) error {
	parts := strings.Split(module, ".")
	filename := parts[len(parts)-1] + "Spec.js"
	p := filepath.Join("client", "test", "unit", "controllers", filename)

	return writeFile(p, controllerTestTmpl, &ControllerData{name, module, route})
}
