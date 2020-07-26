package object

import "testing"

// ハッシュのキーには文字列、数値、booleanが使えるようにしている。ここで注意するところがある。
// 下記のコードで出てくる、二つの"name"は、Valueこそ一緒だが異なるStringオブジェクトとして生成されており、挿しているポインタは別物
// -------
// let hash = {"name": "Monkey"}
// hash["name"]
// -------
// HashオブジェクトのPairsのキーには、そのままのオブジェクトを突っ込んで比較してしまうと、hashから値が取り出せない状態になる。
// なぜかというと、Stringオブジェクト、Booleanオブジェクト、Integerオブジェクトが生成される際、オブジェクトのポインタを返すようにしている。（Eval関数を参照）
// なので、そのままのオブジェクトを格納するということは、HashオブジェクトのPairsのキーにはポインタを格納するということになる。
// で、評価の際、Stringオブジェクト、Booleanオブジェクト、Integerオブジェクトは常に新しいオブジェクトを生成する。（ポインタは常に変わる）
// そしてhashから値を取り出す操作はgoのmapをそのまま使って評価するので、ポインタが異なってしまうとhashから値は取り出せない。指す値が同じオブジェクトだとしても関係ない。
//   参考：[Golang]mapのkeyのちょっとした話
//   https://ken-aio.github.io/post/2019/05/28/go-map-tips/
//
// この問題を解消するために、Hashのキーとなりうる3つのオブジェクトにはHashKey()というメソッドをはやし、その結果でキーの値を比較するようにする。
// HashKey()メソッドはHashKey構造体を返すようになっている。各オブジェクトごとにHashKey構造体のValueにはuint64が入るよう実装している。
// - Booleanオブジェクトはtrue/falseによって1/0を入れる
// - IntegerオブジェクトはValueの値(整数)をuint64でcastしたものを入れる。
// - Stringオブジェクトはhash/fnvパッケージを使って、Valueの文字列をハッシュ化した数値を入れる。
// 異なるオブジェクトのHashKeyを比較する際は、このHashKey構造体同士を比較する。（evalHashIndexExpression関数を参照）
// (goではmapのキーに構造体を使うことができる。またキーバリューの値が同じ構造体ならmapから値を取り出せる。)

func TestStringHashKey(t *testing.T) {
	hello1 := &String{Value: "Hello World"}
	hello2 := &String{Value: "Hello World"}
	diff1 := &String{Value: "My name is johnny"}
	diff2 := &String{Value: "My name is johnny"}

	if hello1.HashKey() != hello2.HashKey() {
		t.Errorf("strings with same content have different hash keys")
	}

	if diff1.HashKey() != diff2.HashKey() {
		t.Errorf("strings with same content have different hash keys")
	}

	if hello1.HashKey() == diff1.HashKey() {
		t.Errorf("strings with different content have same hash keys")
	}
}

func TestBooleanHashKey(t *testing.T) {
	true1 := &Boolean{Value: true}
	true2 := &Boolean{Value: true}
	false1 := &Boolean{Value: false}
	false2 := &Boolean{Value: false}

	if true1.HashKey() != true2.HashKey() {
		t.Errorf("trues do not have same hash key")
	}

	if false1.HashKey() != false2.HashKey() {
		t.Errorf("falses do not have same hash key")
	}

	if true1.HashKey() == false1.HashKey() {
		t.Errorf("true has same hash key as false")
	}
}

func TestIntegerHashKey(t *testing.T) {
	one1 := &Integer{Value: 1}
	one2 := &Integer{Value: 1}
	two1 := &Integer{Value: 2}
	two2 := &Integer{Value: 2}

	if one1.HashKey() != one2.HashKey() {
		t.Errorf("integers with same content have twoerent hash keys")
	}

	if two1.HashKey() != two2.HashKey() {
		t.Errorf("integers with same content have twoerent hash keys")
	}

	if one1.HashKey() == two1.HashKey() {
		t.Errorf("integers with twoerent content have same hash keys")
	}
}
