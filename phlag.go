package phlag

import (
	"github.com/coreos/go-etcd/etcd"
	"os"
	"github.com/fatih/structs"
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"net/url"
	"log"
	"time"
)

var flagSet = flag.CommandLine
var flagSetArgs = os.Args[1:]
var durationKind = reflect.TypeOf(time.Nanosecond).Kind()

const (
	phlagTag        = "phlag"
	descriptionTag  = "description"
	etcdEndpointVar = "ETCD_ENDPOINT"
)

type Phlag struct {
	client           etcdClient
	etcdPathTemplate string // Etcd location of param, for example '/company.com/config/%v'

}

// Minimal interface definition around etcd client, allows creation of fake in tests
type etcdClient interface {
	Get(string, bool, bool) (*etcd.Response, error)
}

func New(template string) (*Phlag, error) {
	return NewFromEnvironment(etcdEndpointVar, template)
}

func NewFromEnvironment(envName string, template string) (*Phlag, error) {
	etcdHostEnv := os.Getenv(envName)
	if etcdHostEnv == "" {
		return &Phlag{}, nil
	}

	parsedEtcdUrl, err := url.Parse(etcdHostEnv)
	if err != nil {
		return nil, fmt.Errorf("%v environment variable does not contain a valid url", envName)
	}

	return NewWithEndpoint(parsedEtcdUrl, template)
}

func NewWithEndpoint(endpoint *url.URL, template string) (*Phlag, error) {
	if !endpoint.IsAbs() {
	 	return nil, fmt.Errorf("endpoint '%v' is not an absolute url ( http://foo.com:4001 )", endpoint.String())
	}

	log.Printf("Using etcd endpoint : %v", endpoint.String())
	client := etcd.NewClient([]string{endpoint.String()})
	client.SetConsistency(etcd.WEAK_CONSISTENCY)
	return NewWithClient(client, template), nil
}

func NewWithClient(client etcdClient, template string) *Phlag {
	return &Phlag{client, template }
}

// Get the named parameter from either the cli or etcd
func (e *Phlag) Get(name string) *string {
	if flagGiven(flagSet, name) {
		valueFromCli := flagSet.Lookup(name)
		log.Printf("Using command line value %v for param %v", valueFromCli.Value.String(), name)
		cliValue := valueFromCli.Value.String()
		return &cliValue
	}

	if e.client == nil {
		return nil
	}

	// No command line param given, lookup through etcd
	// log.Printf("Fetching param %v from etcd", name)
	etcPath := fmt.Sprintf(e.etcdPathTemplate, name)
	// log.Printf("Using etc path %v", etcPath)
	valueFromEtcd, err := e.client.Get(etcPath, false, false)
	if err != nil { // TODO : Sort out '100: Key not found' messages
		log.Printf(err.Error())
		return nil
	}
	if valueFromEtcd.Node != nil {
		// log.Printf("Returing node value %v", valueFromEtcd.Node.Value)
		log.Printf("Using etcd value %v for param %v", valueFromEtcd.Node.Value, name)
		return &valueFromEtcd.Node.Value
	}
	return nil
}

func (e *Phlag) Resolve(target interface{}) {
	s := structs.New(target)
	for _, field := range s.Fields() {
		configuredName := field.Tag(phlagTag)
		description := field.Tag(descriptionTag)
		switch field.Kind() {
		case durationKind: flagSet.String(configuredName, field.Value().(time.Duration).String(), description)
		case reflect.String : flagSet.String(configuredName, field.Value().(string), description)
		case reflect.Int : flagSet.Int(configuredName, field.Value().(int), description)
		}

	}
	flagSet.Parse(flagSetArgs)
	for _, field := range s.Fields() {
		configuredName := field.Tag(phlagTag)
		resolvedValue := e.Get(configuredName)
		if resolvedValue == nil {
			log.Printf("Cannot resolve field %v using cli params or etcd", configuredName)
			continue
		}
		// log.Printf("Field %v is of type %v, setting resolved value %v", field.Name(), field.Kind().String(), *resolvedValue)
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
			log.Printf("Unable to handle reflect.Kind : %v", field.Kind())
		}
		if err != nil {
			log.Printf("Could not set field %v, encoutered error %v", field.Name(), err.Error())
		}
		//log.Printf("Field %v now has value %v", field.Name(), field.Value())
	}
}

func flagGiven(flagSet *flag.FlagSet, name string) bool {
	var flags = []string{}
	flagSet.Visit(func(f *flag.Flag) { flags = append(flags, f.Name)})
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
