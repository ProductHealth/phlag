# phlag

Command line configuration library with etcd fallback.

# Configuration
## Etcd
By default phlag uses the etcd endpoint as define int he ETCD_ENDPOINT environment variable.
In order to use a different variable or your own etcd client use one of the alternative methods in the phlag package.

## Logging
Phlag logs to the sdk logger by default, if you prefer a different logging framework overwrite phlag.Logger with the implementation you prefer. 

# Usage

```
import (
	"github.com/ProductHealth/phlag"
)

type MyConfiguration struct {
    SomeField       string      `phlag:"somefield"`
}

func ReadConfiguration() *Configuration {
	p, _ := phlag.New("/company.com/services/fooservice/params/%v")
	var config = new(Configuration)
	p.Resolve(config)
	return config
}

```

phlag will attempt to find a command line argument 'somefield', if this is not present phlag will query etcd using the given template.