package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/ahilsend/vaultify/pkg/vault"
	"github.com/hashicorp/go-hclog"
	"io"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/Masterminds/sprig"

	"github.com/ahilsend/vaultify/pkg/secrets"
)

const (
	templateName = "vaultify"
)

type VaultifyTemplate struct {
	secretReader secrets.SecretReader
	logger       hclog.Logger
}

func Run(logger hclog.Logger, options *Options) error {
	secretReader, err := createSecretReader(logger, options)
	if err != nil {
		return err
	}

	vaultTemplate := New(logger, secretReader)
	resultSecrets, err := vaultTemplate.RenderToFile(options.TemplateFileName, options.OutputFileName)
	if err != nil {
		return err
	}

	if options.SecretsOutputFileName == "" {
		return nil
	}
	return secrets.Write(options.SecretsOutputFileName, resultSecrets)
}

func createSecretReader(logger hclog.Logger, options *Options) (secrets.SecretReader, error) {
	if len(options.Variables) > 0 {
		values := secrets.MapSecrets{}
		for name, jsonString := range options.Variables {
			var secret secrets.Value
			err := json.Unmarshal([]byte(jsonString), &secret)
			if err != nil {
				return nil, err
			}
			values[name] = secret
		}
		return secrets.NewMapReader(values), nil
	}

	config := options.VaultApiConfig()
	vaultClient, err := vault.NewClient(logger, options.Role, config)
	if err != nil {
		return nil, err
	}

	return secrets.NewVaultReader(vaultClient), nil
}

func New(logger hclog.Logger, secretReader secrets.SecretReader) *VaultifyTemplate {
	return &VaultifyTemplate{
		secretReader: secretReader,
		logger:       logger,
	}
}

func (t *VaultifyTemplate) getVaultSecret(name string) (*secrets.Secret, error) {
	if name == "" {
		return nil, errors.New("you need to pass a name to the 'vault' function")
	}

	return t.secretReader.Get(name)
}

func (t *VaultifyTemplate) RenderToFile(templateFile string, outputFile string) (*secrets.Secrets, error) {
	t.logger.Info("Rendering template", "template", templateFile)
	templateBytes, err := ioutil.ReadFile(templateFile)
	if err != nil {
		return nil, err
	}

	var output io.Writer
	if outputFile == "" {
		output = os.Stdout
	} else {
		file, err := os.Create(outputFile)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		file.Chmod(0600)
		output = file
	}

	resultSecrets, err := t.render(bytes.NewBuffer(templateBytes), output)
	if err != nil {
		t.logger.Error("Error during rendering", "error", err)
		return nil, err
	}

	return resultSecrets, nil
}

func (t *VaultifyTemplate) render(input io.Reader, output io.Writer) (*secrets.Secrets, error) {
	inputBytes, err := ioutil.ReadAll(input)
	if err != nil {
		return nil, err
	}

	result := secrets.Secrets{
		AuthSecret: t.secretReader.GetAuthSecret(),
		Secrets:    map[string]secrets.Secret{},
	}
	tmpl := template.New(templateName)
	tmpl.Delims("<{", "}>")
	funcMap := sprig.GenericFuncMap()
	funcMap["vault"] = func(name string) (*secrets.Secret, error) {
		secret, err := t.getVaultSecret(name)
		if err != nil {
			return nil, err
		}

		result.Secrets[name] = *secret

		return secret, err
	}
	tmpl.Funcs(funcMap)

	_, err = tmpl.Parse(string(inputBytes))
	if err != nil {
		return nil, err
	}

	err = tmpl.Execute(output, nil)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
