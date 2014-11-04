package phlag

import (
	"flag"
	"fmt"
	getcd "github.com/ProductHealth/gommons/etcd"
	"github.com/coreos/go-etcd/etcd"
	"github.com/fatih/structs"
	"log"
	"os"
	"reflect"
	"strconv"
	"time"
)

var flagSet = flag.CommandLine
var flagSetArgs = os.Args[1:]
var durationKind = reflect.TypeOf(time.Nanosecond).Kind()

const (
	etcdTag        = "etcd"
	phlagTag       = "phlag"
	descriptionTag = "description"
)

type Phlag struct {
	client           etcdClient
	etcdPathTemplate string // Etcd location of param, for example '/company.com/config/%v'

}

// Logger function, replace by your preferred implementation
var Logger func(string, ...interface{}) = log.Printf

// Minimal interface definition around etcd client, allows creation of fake in tests
type etcdClient interface {
	Get(string, bool, bool) (*etcd.Response, error)
}

func New(template string) (*Phlag, error) {
	client, err := getcd.NewEtcdClient()
	if err != nil {
		return nil, err
	}
	return NewWithClient(client, template), nil
}

func NewWithClient(client etcdClient, template string) *Phlag {
	return &Phlag{client, template}
}

// Get the named parameter from either the cli or etcd
func (e *Phlag) Get(name, etcdPath string) *string {
	if flagGiven(flagSet, name) {
		valueFromCli := flagSet.Lookup(name)
		Logger("Using command line value %v for param %v", valueFromCli.Value.String(), name)
		cliValue := valueFromCli.Value.String()
		return &cliValue
	}

	if e.client == nil {
		return nil
	}

	// No command line param given, lookup through etcd
	// Logger("Fetching param %v from etcd", name)
	if etcdPath == "" {
		etcdPath = fmt.Sprintf(e.etcdPathTemplate, name)
	}
	// Logger("Using etc path %v", etcdPath)
	valueFromEtcd, err := e.client.Get(etcdPath, false, false)
	if err != nil { // TODO : Sort out '100: Key not found' messages
		Logger(err.Error())
		return nil
	}
	if valueFromEtcd.Node != nil {
		// Logger("Returing node value %v", valueFromEtcd.Node.Value)
		Logger("Using etcd value %v for param %v", valueFromEtcd.Node.Value, name)
		return &valueFromEtcd.Node.Value
	}
	return nil
}

func (e *Phlag) Resolve(target interface{}) {
	s := structs.New(target)
	for _, field := range s.Fields() {
		configuredName := field.Tag(phlagTag)
		if configuredName == "" {
			continue
		}
		description := field.Tag(descriptionTag)
		switch field.Kind() {
		case durationKind:
			flagSet.String(configuredName, field.Value().(time.Duration).String(), description)
		case reflect.String:
			flagSet.String(configuredName, field.Value().(string), description)
		case reflect.Int:
			flagSet.Int(configuredName, field.Value().(int), description)
		}

	}
	flagSet.Parse(flagSetArgs)
	for _, field := range s.Fields() {
		configuredName := field.Tag(phlagTag)
		if configuredName == "" {
			continue
		}
		etcdPath := field.Tag(etcdTag)
		resolvedValue := e.Get(configuredName, etcdPath)
		if resolvedValue == nil {
			Logger("Cannot resolve field %v using cli params or etcd", configuredName)
			continue
		}
		// Logger("Field %v is of type %v, setting resolved value %v", field.Name(), field.Kind().String(), *resolvedValue)
		var err error
		switch {
		case field.Kind() == durationKind:
			v := *resolvedValue
			d, err := time.ParseDuration(v)
			if err == nil {
				err = field.Set(d)
			}
		case field.Kind() == reflect.String:
			v := *resolvedValue
			err = field.Set(v)
		case field.Kind() == reflect.Int:
			v, _ := strconv.Atoi(*resolvedValue)
			err = field.Set(v)
		case field.Kind() == reflect.Int32:
			v, _ := strconv.Atoi(*resolvedValue)
			err = field.Set(v)
		case field.Kind() == reflect.Int64:
			v, _ := strconv.Atoi(*resolvedValue)
			err = field.Set(v)
		default:
			Logger("Unable to handle reflect.Kind : %v", field.Kind())
		}
		if err != nil {
			Logger("Could not set field %v, encoutered error %v", field.Name(), err.Error())
		}
		//Logger("Field %v now has value %v", field.Name(), field.Value())
	}
}

func flagGiven(flagSet *flag.FlagSet, name string) bool {
	var flags = []string{}
	flagSet.Visit(func(f *flag.Flag) { flags = append(flags, f.Name) })
	return stringInSlice(name, flags)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
