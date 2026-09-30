package config

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

const (
	// Control variables: they steer loading and are never configuration keys.
	envProfile   = "NWIKI_ENV"
	envConfigDir = "NWIKI_CONFIG_DIR"

	envPrefix = "NWIKI_"
	envKeySep = "__" // separates levels: NWIKI_DATABASE__MAX_CONNS -> database.max_conns
	baseFile  = "config.yaml"
)

// Sources tells Load where configuration comes from.
type Sources struct {
	// Embedded holds the built-in config.yaml and config.<env>.yaml files.
	Embedded fs.FS
	// Environ is the process environment in os.Environ form.
	Environ []string
	// LocalFile is the personal override file. It is read only in the dev
	// profile, and only when it exists.
	LocalFile string
}

// Load builds the effective configuration. Each layer overrides the ones
// before it:
//
//  1. the built-in config.yaml, which lists every key with its default;
//  2. the built-in config.<env>.yaml;
//  3. config.yaml, then config.<env>.yaml, in $NWIKI_CONFIG_DIR, when set and present;
//  4. LocalFile, in the dev profile only, when present;
//  5. NWIKI_<SECTION>__<KEY> environment variables.
//
// The result is validated; the error lists every invalid key.
func Load(src Sources) (Config, error) {
	profile := lookupEnv(src.Environ, envProfile)
	if profile == "" {
		profile = EnvDev
	}
	if !slices.Contains([]string{EnvDev, EnvTest, EnvProd}, profile) {
		return Config{}, fmt.Errorf("%s must be one of dev, test, prod, got %q", envProfile, profile)
	}
	profileFile := "config." + profile + ".yaml"

	k := koanf.New(".")
	for _, name := range []string{baseFile, profileFile} {
		data, err := fs.ReadFile(src.Embedded, name)
		if err != nil {
			return Config{}, fmt.Errorf("read built-in %s: %w", name, err)
		}
		if err := k.Load(rawbytes.Provider(data), yaml.Parser()); err != nil {
			return Config{}, fmt.Errorf("parse built-in %s: %w", name, err)
		}
	}
	if dir := lookupEnv(src.Environ, envConfigDir); dir != "" {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			return Config{}, fmt.Errorf("%s=%s is not a directory", envConfigDir, dir)
		}
		for _, name := range []string{baseFile, profileFile} {
			if err := loadFileIfExists(k, filepath.Join(dir, name)); err != nil {
				return Config{}, err
			}
		}
	}
	if profile == EnvDev && src.LocalFile != "" {
		if err := loadFileIfExists(k, src.LocalFile); err != nil {
			return Config{}, err
		}
	}
	environ := env.Provider(".", env.Opt{
		Prefix:        envPrefix,
		TransformFunc: envKey,
		EnvironFunc:   func() []string { return src.Environ },
	})
	if err := k.Load(environ, nil); err != nil {
		return Config{}, fmt.Errorf("read environment: %w", err)
	}
	if err := rejectNulls(k); err != nil {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", err)
	}

	cfg := Config{Env: profile}
	if err := decode(k, &cfg); err != nil {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", err)
	}
	return cfg, nil
}

func lookupEnv(environ []string, name string) string {
	var value string
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			value = v
		}
	}
	return value
}

// loadFileIfExists merges the YAML file at path into k and skips a missing
// file. It reads the file itself rather than through koanf's file provider,
// which would link fsnotify into the binary for a watch nervewiki never uses.
func loadFileIfExists(k *koanf.Koanf, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	if err := k.Load(rawbytes.Provider(data), yaml.Parser()); err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	return nil
}

// envKey maps NWIKI_DATABASE__MAX_CONNS to database.max_conns. Variables
// without the "__" separator (NWIKI_ENV, NWIKI_CONFIG_DIR, the compose file's
// NWIKI_DEV_DB_PORT and the like) are not configuration keys and are skipped:
// every key lives in a section, so a real key always has a separator.
func envKey(name, value string) (string, any) {
	key := strings.TrimPrefix(name, envPrefix)
	if !strings.Contains(key, envKeySep) {
		return "", nil
	}
	return strings.ToLower(strings.ReplaceAll(key, envKeySep, ".")), value
}

// rejectNulls fails every key that a layer left without a value, and every
// null inside a list. The decoder would skip a null key and leave it at its
// zero value (auto_migrate: in a YAML file would silently mean false,
// whatever the layers below it say), and turn a null list item into a zero
// item. koanf flattens the maps of a layer into keys but keeps a list, with
// whatever it holds, as one value, so each value is searched too.
func rejectNulls(k *koanf.Koanf) error {
	var errs []error
	for _, key := range k.Keys() {
		errs = appendNulls(errs, key, k.Get(key))
	}
	return errors.Join(errs...)
}

// appendNulls appends an error for value if it is null, and for every null
// inside it if it is a list or a map, each named by its path: key[1] for a
// list item, key.name for a map entry.
func appendNulls(errs []error, path string, value any) []error {
	if value == nil {
		return append(errs, fmt.Errorf("%s: must not be null (a key without a value in YAML)", path))
	}
	switch v := reflect.ValueOf(value); v.Kind() {
	case reflect.Slice:
		for i := range v.Len() {
			errs = appendNulls(errs, fmt.Sprintf("%s[%d]", path, i), v.Index(i).Interface())
		}
	case reflect.Map: // YAML keys need not be strings: order them as printed
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
		for _, key := range keys {
			errs = appendNulls(errs, fmt.Sprintf("%s.%v", path, key), v.MapIndex(key).Interface())
		}
	}
	return errs
}

// decode copies the merged layers into cfg. Unknown keys are errors, so a typo
// never silently falls back to a default.
func decode(k *koanf.Koanf, cfg *Config) error {
	return k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				emptyValueHook, durationHook, numberHook),
			ErrorUnused:      true,
			WeaklyTypedInput: true, // environment values are strings
		},
	})
}

// emptyValueHook rejects an empty string for a key that is not a string.
// Weakly typed decoding would otherwise turn NWIKI_DATABASE__AUTO_MIGRATE=
// into false and NWIKI_DATABASE__MAX_CONNS= into 0 without a word.
func emptyValueHook(from, to reflect.Type, data any) (any, error) {
	if from.Kind() != reflect.String || to.Kind() == reflect.String || data != "" {
		return data, nil
	}
	switch to.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return nil, errors.New("must not be empty")
	}
	return data, nil
}

// durationHook decodes Go duration strings such as "5s". Bare numbers are
// rejected: a YAML 5 would otherwise silently mean 5ns.
func durationHook(_ reflect.Type, to reflect.Type, data any) (any, error) {
	if to != reflect.TypeFor[time.Duration]() {
		return data, nil
	}
	s, ok := data.(string)
	if !ok {
		return nil, fmt.Errorf("must be a duration such as \"5s\", got %v", data)
	}
	return time.ParseDuration(s)
}

// numberHook rejects a YAML number that the key's integer type cannot hold.
// Weakly typed decoding would convert it anyway: -1 would wrap to the largest
// value of an unsigned key, max_conns: 5000000000 would lose its high bits and
// 1.5 would become 1. Environment values are strings, which the decoder
// parses with the type's bit size, so they pass through here.
func numberHook(_ reflect.Type, to reflect.Type, data any) (any, error) {
	var lo int64
	var hi uint64
	// above is hi+1, the first number past the type. A float is compared with
	// it rather than with hi: float64 holds every power of two exactly, but
	// rounds a 64-bit hi up to above, which would let above itself through.
	var above float64
	switch to.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		lo, hi = -1<<(to.Bits()-1), 1<<(to.Bits()-1)-1
		above = math.Ldexp(1, to.Bits()-1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		hi = math.MaxUint64 >> (64 - to.Bits())
		above = math.Ldexp(1, to.Bits())
	default:
		return data, nil
	}
	if to == reflect.TypeFor[time.Duration]() { // durationHook has decoded it
		return data, nil
	}
	var fits bool
	switch v := reflect.ValueOf(data); {
	case v.CanInt():
		fits = v.Int() >= lo && (v.Int() < 0 || uint64(v.Int()) <= hi)
	case v.CanUint():
		fits = v.Uint() <= hi
	case v.CanFloat():
		f := v.Float()
		fits = f == math.Trunc(f) && f >= float64(lo) && f < above // float64(lo) is exact: -2^(bits-1), or 0
	default:
		return data, nil
	}
	if !fits {
		return nil, fmt.Errorf("must be a whole number from %d to %d, got %v", lo, hi, data)
	}
	return data, nil
}
