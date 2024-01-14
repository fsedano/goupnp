package envelope

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/fsedano/goupnp/v2alpha/soap/types"
	"github.com/google/go-cmp/cmp"
)

type testStructArgs struct {
	Foo string
	Bar string
}

type newString string

// TestWriteRead tests the round-trip of writing an envelope and reading it back.
func TestWriteRead(t *testing.T) {
	tests := []struct {
		name    string
		argsIn  any
		argsOut any
	}{
		{
			"struct",
			&testStructArgs{
				Foo: "foo-1",
				Bar: "bar-2",
			},
			&testStructArgs{},
		},
		{
			"mapString",
			map[string]string{
				"Foo": "foo-1",
				"Bar": "bar-2",
			},
			map[string]string{},
		},
		{
			"mapUI2",
			map[string]types.UI2{
				"Foo": 1,
				"Bar": 2,
			},
			map[string]types.UI2{},
		},
		{
			"mapNewStringKey",
			map[newString]string{
				"Foo": "foo-1",
				"Bar": "bar-2",
			},
			map[newString]string{},
		},
	}

	for _, test := range tests {
		test := test // copy for closure
		t.Run(test.name, func(t *testing.T) {
			actionIn := NewSendAction("urn:schemas-upnp-org:service:FakeService:1", "MyAction", test.argsIn)

			buf := &bytes.Buffer{}
			err := Write(buf, actionIn)
			if err != nil {
				t.Fatalf("Write want success, got err=%v", err)
			}

			actionOut := NewRecvAction(test.argsOut)

			err = Read(buf, actionOut)
			if err != nil {
				t.Errorf("Read want success, got err=%v", err)
			}

			if diff := cmp.Diff(actionIn, actionOut); diff != "" {
				t.Errorf("\nwant actionOut=%+v\ngot  %+v\ndiff:\n%s", actionIn, actionOut, diff)
			}
			if diff := cmp.Diff(test.argsIn, test.argsOut); diff != "" {
				t.Errorf("\nwant argsOut=%+v\ngot  %+v\ndiff:\n%s", test.argsIn, test.argsOut, diff)
			}
			if t.Failed() {
				t.Logf("Encoded envelope:\n%v", buf)
			}
		})
	}
}

// TestRead tests read against a semi-real encoded envelope.
func TestRead(t *testing.T) {
	env := []byte(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>
<u:FakeAction xmlns:u="urn:schemas-upnp-org:service:FakeService:1">
<Foo>foo-1</Foo>
<Bar>bar-2</Bar>
</u:FakeAction>
</s:Body> </s:Envelope>`)
	argsOut := &testStructArgs{}

	actionOut := NewRecvAction(argsOut)

	if err := Read(bytes.NewBuffer(env), actionOut); err != nil {
		t.Fatalf("Read want success, got err=%v", err)
	}

	wantArgsOut := &testStructArgs{
		Foo: "foo-1",
		Bar: "bar-2",
	}

	if diff := cmp.Diff(wantArgsOut, argsOut); diff != "" {
		t.Errorf("want argsOut=%+v, got %+v\ndiff:\n%s", wantArgsOut, argsOut, diff)
	}
}

func TestReadFault(t *testing.T) {
	env := []byte(xml.Header + `
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<s:Body>
<s:Fault>
<faultcode>dummy code</faultcode>
<faultstring>dummy string</faultstring>
<faultactor>dummy actor</faultactor>
<detail>dummy detail</detail>
</s:Fault>
</s:Body>
</s:Envelope>
`)

	err := Read(bytes.NewBuffer(env), NewRecvAction(&testStructArgs{}))
	if err == nil {
		t.Fatal("want err != nil, got nil")
	}

	gotFault, ok := err.(*Fault)
	if !ok {
		t.Fatalf("want *Fault, got %T", err)
	}

	wantFault := &Fault{
		Code:   "dummy code",
		String: "dummy string",
		Actor:  "dummy actor",
		Detail: FaultDetail{Raw: []byte("dummy detail")},
	}
	if !reflect.DeepEqual(wantFault, gotFault) {
		t.Errorf("want %+v, got %+v", wantFault, gotFault)
	}
}

func TestFault(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		wantIs bool
	}{
		{"plain fault", &Fault{Code: "code"}, true},
		{"wrapped fault", fmt.Errorf("wrapper: %w", &Fault{Code: "code"}), true},
		{"other error", io.EOF, false},
	}

	for _, test := range tests {
		test := test // copy for closure
		t.Run(test.name, func(t *testing.T) {
			if got, want := errors.Is(test.err, ErrFault), test.wantIs; got != want {
				t.Errorf("got errors.Is(%v, ErrFault)=>%t, want %t", test.err, got, want)
			}
			if !test.wantIs {
				return
			}

			var fault *Fault
			if got, want := errors.As(test.err, &fault), true; got != want {
				t.Fatalf("got errors.As(%v, ...)=>%t, want %t", test.err, got, want)
			}

			if got, want := fault.Code, "code"; got != want {
				t.Errorf("got fault.Code=%q, want %q", got, want)
			}
		})
	}
}
