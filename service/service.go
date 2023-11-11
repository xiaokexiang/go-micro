package service

import (
	"errors"
	"reflect"
)

type Message struct {
	MethodName   string
	Args, Result int
}

type Service struct {
	typ    reflect.Type           // 结构体类型
	self   reflect.Value          // 结构体实例本身
	Method map[string]*MethodType // 结构体中所有符合条件的方法
}

type MethodType struct {
	Method reflect.Method
	Args   reflect.Type
	Result reflect.Type
}

func NewService(v any) *Service {
	s := new(Service)
	s.self = reflect.ValueOf(v)
	s.typ = reflect.TypeOf(v)
	s.Method = make(map[string]*MethodType)
	s.registerMethods() // 注册service中的方法
	return s
}

func (s *Service) registerMethods() {
	t := s.typ
	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		if !method.IsExported() {
			continue
		}
		mType := method.Type
		if 2 != mType.NumIn() || 1 != mType.NumOut() { // 第一个参数是方法本身
			continue
		}
		s.Method[method.Name] = &MethodType{
			Method: method,
			Args:   mType.In(0),
			Result: mType.Out(0),
		}
	}
}
func (s *Service) Exec(msg *Message) error {
	name := msg.MethodName
	v, ok := s.Method[name]
	if !ok {
		return errors.New("[Service] Can't find method: " + name)
	}
	res := v.Method.Func.Call([]reflect.Value{s.self, reflect.ValueOf(msg.Args)})
	msg.Result = int(res[0].Int())
	return nil
}

func (s *Service) Square(args int) int {
	return args * args
}
