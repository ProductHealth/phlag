package phlag

import (
	"errors"
	"flag"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type testStruct struct {
	StringField   string        `phlag:"s"`
	IntField      int           `phlag:"i"`
	DurationField time.Duration `phlag:"d"`
}

const (
	testService  = "testService"
	PathTemplate = "/test/%v"
)

type fakeEtcdClient struct {
	values map[string]etcd.Response
}

func newFakeEtcdClient() *fakeEtcdClient {
	fake := fakeEtcdClient{map[string]etcd.Response{}}
	return &fake
}

func StringNodeResponse(value string) etcd.Response {
	return etcd.Response{
		Action: "GET",
		Node: &etcd.Node{
			Key:           "",
			Value:         value,
			Dir:           false,
			Expiration:    nil,
			TTL:           0,
			Nodes:         etcd.Nodes{},
			ModifiedIndex: 0,
			CreatedIndex:  0,
		},
		PrevNode:  nil,
		EtcdIndex: 0,
		RaftIndex: 0,
		RaftTerm:  0,
	}
}

func (f *fakeEtcdClient) Set(path string, value string) {
	f.values[path] = StringNodeResponse(value)
}

func (f *fakeEtcdClient) Get(path string, _ bool, _ bool) (*etcd.Response, error) {
	if val, ok := f.values[path]; ok {
		return &val, nil
	} else {
		return nil, errors.New("Not found in fake")
	}
}

func TestFlagGivenValidatesGivenFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	fs.String("test", "test", "test")
	fs.String("test2", "test2", "test2")
	fs.Parse([]string{"--test", "foo"})

	assert.True(t, flagGiven(fs, "test"))
	assert.False(t, flagGiven(fs, "test2"))
}

func TestGetResolvesCliParamsFirst(t *testing.T) {
	// Setup flag
	flagSet = flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("param1", "param1", "param1")
	flagSet.Parse([]string{"--param1", "valuefromflag"})

	// Setup etcd
	fakeClient := newFakeEtcdClient()
	fakeClient.Set(fmt.Sprintf(PathTemplate, "param1"), "valuefrometc")
	ph := NewWithClient(fakeClient, "/params%v")
	param1 := ph.Get("param1")
	assert.Equal(t, "valuefromflag", *param1)
}

func TestGetResolvesEtcdIfCliFails(t *testing.T) {
	// Setup flag
	flagSet = flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("param1", "param1", "param1")
	flagSet.Parse([]string{"--param1", "valuefromflag"})

	// Setup etcd
	fakeClient := newFakeEtcdClient()
	fakeClient.Set(fmt.Sprintf(PathTemplate, "param2"), "valuefrometc")
	ph := NewWithClient(fakeClient, PathTemplate)
	param1 := ph.Get("param1")
	param2 := ph.Get("param2")
	assert.Equal(t, "valuefromflag", *param1)
	assert.Equal(t, "valuefrometc", *param2)
}

func TestGetReturnsNilIfKeyNotAvailable(t *testing.T) {
	// Setup flag
	flagSet = flag.NewFlagSet("test1", flag.ContinueOnError)
	flagSet.Parse([]string{})

	// Setup etcd
	fakeClient := newFakeEtcdClient()
	ph := NewWithClient(fakeClient, PathTemplate)
	param1 := ph.Get("param1")
	assert.Nil(t, param1)
}

func TestResolvesPopulatesStringFields(t *testing.T) {
	// Setup flag
	flagSet = flag.NewFlagSet("", flag.ContinueOnError)
	flagSetArgs = []string{"--s", "stringvalue", "--i", "123", "--d", "5m"}

	// Setup etcd
	fakeClient := newFakeEtcdClient()
	ph := NewWithClient(fakeClient, PathTemplate)

	s := &testStruct{}

	ph.Resolve(s)
	assert.Equal(t, s.DurationField, time.Minute*5)
	assert.Equal(t, s.StringField, "stringvalue")
	assert.Equal(t, s.IntField, 123)
}

func TestResolveAllowsMissingFields(t *testing.T) {
	// Setup flag
	flagSet = flag.NewFlagSet("", flag.ContinueOnError)
	flagSetArgs = []string{"--s", "stringvalue"}

	// Setup etcd
	fakeClient := newFakeEtcdClient()
	ph := NewWithClient(fakeClient, PathTemplate)

	s := &testStruct{}

	ph.Resolve(s)
	assert.Equal(t, "stringvalue", s.StringField)
	assert.Equal(t, 0, s.IntField)
	assert.Equal(t, 0, s.DurationField)
}
