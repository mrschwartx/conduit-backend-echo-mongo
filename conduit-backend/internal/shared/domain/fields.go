package domain

type Fields map[string]Raw

func (f Fields) Get(key string) Raw {
	return f[key]
}
