package config

import "os"

func osLookupEnv(k string) (string, bool) { return os.LookupEnv(k) }
