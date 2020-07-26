package object

// 現在のenvで、新しいenvを囲い込む。現在のenvが外側のスコープとなるイメージ。
// 現在のenvは引数で渡されているouter。
// つまりスコープがネストするごとに内側にenvがネストされていくイメージ。
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil} // ルートのスコープにはouterスコープはない。
}

type Environment struct {
	store map[string]Object
	outer *Environment
}

// 内側のスコープで見つからないなら外側のスコープで探す。それを再帰的に行う。
// 一番外側のスコープまでいった時はそれはルートスコープ（NewEnvironmentで作った環境）
// （envをスコープごとに区切ることで、クロージャを実現することができる）
func (e *Environment) Get(name string) (Object, bool) {
	//j, _ := json.MarshalIndent(e.store, "", " ")
	//fmt.Printf("現在のstore内容=================\n%v\n", string(j))

	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val

	//j, _ := json.MarshalIndent(e.store, "", " ")
	//fmt.Printf("store結果=================\n%v\n", string(j))
	return val
}
