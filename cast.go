package deferred

import "encoding/json"

func Cast[T any](unmarshal func(any, *T) error) func(any, error) (*T, error) {
	return func(value any, err error) (*T, error) {
		if err != nil {
			return nil, err
		}
		if v, ok := value.(*T); ok {
			return v, nil
		}
		if v, ok := value.(T); ok {
			return &v, nil
		}

		var v T
		err = unmarshal(value, &v)
		return &v, err
	}
}

func CastJson[T any](value any, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	if v, ok := value.(*T); ok {
		return v, nil
	}
	if v, ok := value.(T); ok {
		return &v, nil
	}

	var v T
	switch data := value.(type) {
	case string:
		err = json.Unmarshal([]byte(data), &v)
		return &v, err
	case []byte:
		err = json.Unmarshal(data, &v)
		return &v, err
	case *[]byte:
		err = json.Unmarshal(*data, &v)
		return &v, err
	default:
		var bytes []byte
		if bytes, err = json.Marshal(data); err != nil {
			return nil, err
		}
		err = json.Unmarshal(bytes, &v)
		return &v, err
	}
}
