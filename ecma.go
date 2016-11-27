package colfer

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const ecmaKeywords = "break case catch class const continue debugger default delete do else enum export extends finally for function if import in instanceof new return super switch this throw try typeof var void while with yield"

func IsECMAKeyword(s string) bool {
	for _, k := range strings.Fields(ecmaKeywords) {
		if k == s {
			return true
		}
	}
	return false
}

// GenerateECMA writes the code into file "Colfer.js".
func GenerateECMA(basedir string, packages []*Package) error {
	for _, p := range packages {
		p.NameNative = strings.Replace(p.Name, "/", "_", -1)
		if IsECMAKeyword(p.NameNative) {
			p.NameNative += "_"
		}

		for _, s := range p.Structs {
			for _, f := range s.Fields {
				f.NameNative = f.name
				if IsECMAKeyword(f.NameNative) {
					f.NameNative += "_"
				}
			}
		}
	}

	t := template.New("ecma-code")
	template.Must(t.Parse(ecmaCode))
	template.Must(t.New("marshal").Parse(ecmaMarshal))
	template.Must(t.New("unmarshal").Parse(ecmaUnmarshal))

	if err := os.MkdirAll(basedir, os.ModeDir|os.ModePerm); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(basedir, "Colfer.js"))
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, packages)
}

const ecmaCode = `// This file was generated by colf(1); DO NOT EDIT
{{- range .}}
// The compiler used schema file {{.SchemaFileList}} for package {{.Name}}.
{{- end}}
{{range .}}
var {{.NameNative}} = new function() {
	const EOF = 'colfer: EOF';

	// The upper limit for serial byte sizes.
	var colferSizeMax = 16 * 1024 * 1024;
{{- if .HasList}}
	// The upper limit for the number of elements in a list.
	var colferListMax = 64 * 1024;
{{- end}}
{{range .Structs}}
	// Constructor.
	// When init is provided all enumerable properties are merged into the new object a.k.a. shallow cloning.
	this.{{.NameTitle}} = function(init) {
{{- range .Fields}}
		this.{{.NameNative}} = {{if .TypeList}}[]
{{- else if eq .Type "bool"}}false
{{- else if eq .Type "timestamp"}}null;
		this.{{.NameNative}}_ns = 0
{{- else if eq .Type "text"}}''
{{- else if eq .Type "binary"}}new Uint8Array(0)
{{- else if .TypeRef}}null
{{- else}}0
{{- end}};{{end}}

		for (var p in init) this[p] = init[p];
	}
{{template "marshal" .}}
{{template "unmarshal" .}}
{{end}}
	// private section

	var encodeVarint = function(bytes, x) {
		while (x > 127) {
			bytes.push(x|128);
			x /= 128;
		}
		bytes.push(x&127);
		return bytes;
	}

	var encodeUTF8 = function(s) {
		var i = 0;
		var bytes = new Uint8Array(s.length * 4);
		for (var ci = 0; ci != s.length; ci++) {
			var c = s.charCodeAt(ci);
			if (c < 128) {
				bytes[i++] = c;
				continue;
			}
			if (c < 2048) {
				bytes[i++] = c >> 6 | 192;
			} else {
				if (c > 0xd7ff && c < 0xdc00) {
					if (++ci == s.length) throw 'UTF-8 encode: incomplete surrogate pair';
					var c2 = s.charCodeAt(ci);
					if (c2 < 0xdc00 || c2 > 0xdfff) throw 'UTF-8 encode: second char code 0x' + c2.toString(16) + ' at index ' + ci + ' in surrogate pair out of range';
					c = 0x10000 + ((c & 0x03ff) << 10) + (c2 & 0x03ff);
					bytes[i++] = c >> 18 | 240;
					bytes[i++] = c>> 12 & 63 | 128;
				} else { // c <= 0xffff
					bytes[i++] = c >> 12 | 224;
				}
				bytes[i++] = c >> 6 & 63 | 128;
			}
			bytes[i++] = c & 63 | 128;
		}
		return bytes.subarray(0, i);
	}

	var decodeUTF8 = function(bytes) {
		var s = '';
		var i = 0;
		while (i < bytes.length) {
			var c = bytes[i++];
			if (c > 127) {
				if (c > 191 && c < 224) {
					if (i >= bytes.length) throw 'UTF-8 decode: incomplete 2-byte sequence';
					c = (c & 31) << 6 | bytes[i] & 63;
				} else if (c > 223 && c < 240) {
					if (i + 1 >= bytes.length) throw 'UTF-8 decode: incomplete 3-byte sequence';
					c = (c & 15) << 12 | (bytes[i] & 63) << 6 | bytes[++i] & 63;
				} else if (c > 239 && c < 248) {
					if (i+2 >= bytes.length) throw 'UTF-8 decode: incomplete 4-byte sequence';
					c = (c & 7) << 18 | (bytes[i] & 63) << 12 | (bytes[++i] & 63) << 6 | bytes[++i] & 63;
				} else throw 'UTF-8 decode: unknown multibyte start 0x' + c.toString(16) + ' at index ' + (i - 1);
				++i;
			}

			if (c <= 0xffff) s += String.fromCharCode(c);
			else if (c <= 0x10ffff) {
				c -= 0x10000;
				s += String.fromCharCode(c >> 10 | 0xd800)
				s += String.fromCharCode(c & 0x3FF | 0xdc00)
			} else throw 'UTF-8 decode: code point 0x' + c.toString(16) + ' exceeds UTF-16 reach';
		}
		return s;
	}
}
{{end}}`

const ecmaMarshal = `
	// Serializes the object into an Uint8Array.
{{- range .Fields}}{{if .TypeList}}
	// All null entries in property {{.NameNative}} will be replaced with {{if eq .Type "text"}}an empty String{{else if eq .Type "binary"}}an empty Array{{else}}a new {{.TypeRef.Pkg.NameNative}}.{{.TypeRef.NameTitle}}{{end}}.
{{- end}}{{end}}
	this.{{.NameTitle}}.prototype.marshal = function() {
		var segs = [];
{{range .Fields}}{{if eq .Type "bool"}}
		if (this.{{.NameNative}})
			segs.push([{{.Index}}]);
{{else if eq .Type "uint8"}}
		if (this.{{.NameNative}}) {
			if (this.{{.NameNative}} > 255 || this.{{.NameNative}} < 0)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} out of reach: ' + this.{{.NameNative}};
			segs.push([{{.Index}}, this.{{.NameNative}}]);
		}
{{else if eq .Type "uint16"}}
		if (this.{{.NameNative}}) {
			if (this.{{.NameNative}} > 65535 || this.{{.NameNative}} < 0)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} out of reach: ' + this.{{.NameNative}};
			if (this.{{.NameNative}} < 256)
				segs.push([{{.Index}} | 128, this.{{.NameNative}}]);
			else
				segs.push([{{.Index}}, this.{{.NameNative}} >>> 8, this.{{.NameNative}} & 255]);
		}
{{else if eq .Type "uint32"}}
		if (this.{{.NameNative}}) {
			if (this.{{.NameNative}} > 4294967295 || this.{{.NameNative}} < 0)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} out of reach: ' + this.{{.NameNative}};
			if (this.{{.NameNative}} < 0x200000) {
				var seg = [{{.Index}}];
				encodeVarint(seg, this.{{.NameNative}});
				segs.push(seg);
			} else {
				var bytes = new Uint8Array(5);
				bytes[0] = {{.Index}} | 128;
				var view = new DataView(bytes.buffer);
				view.setUint32(1, this.{{.NameNative}});
				segs.push(bytes)
			}
		}
{{else if eq .Type "uint64"}}
		if (this.{{.NameNative}}) {
			if (this.{{.NameNative}} < 0)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} out of reach: ' + this.{{.NameNative}};
			if (this.{{.NameNative}} > Number.MAX_SAFE_INTEGER)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
			if (this.{{.NameNative}} < 0x2000000000000) {
				var seg = [{{.Index}}];
				encodeVarint(seg, this.{{.NameNative}});
				segs.push(seg);
			} else {
				var bytes = new Uint8Array(9);
				bytes[0] = {{.Index}} | 128;
				var view = new DataView(bytes.buffer);
				view.setUint32(1, this.{{.NameNative}} / 0x100000000);
				view.setUint32(5, this.{{.NameNative}} % 0x100000000);
				segs.push(bytes)
			}
		}
{{else if eq .Type "int32"}}
		if (this.{{.NameNative}}) {
			var seg = [{{.Index}}];
			if (this.{{.NameNative}} < 0) {
				seg[0] |= 128;
				if (this.{{.NameNative}} < -2147483648)
					throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds 32-bit range';
				encodeVarint(seg, -this.{{.NameNative}});
			} else {
				if (this.{{.NameNative}} > 2147483647)
					throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds 32-bit range';
				encodeVarint(seg, this.{{.NameNative}});
			}
			segs.push(seg);
		}
{{else if eq .Type "int64"}}
		if (this.{{.NameNative}}) {
			var seg = [4];
			if (this.{{.NameNative}} < 0) {
				seg[0] |= 128;
				if (this.{{.NameNative}} < -Number.MAX_SAFE_INTEGER)
					throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
				encodeVarint(seg, -this.{{.NameNative}});
			} else {
				if (this.{{.NameNative}} > Number.MAX_SAFE_INTEGER)
					throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
				encodeVarint(seg, this.{{.NameNative}});
			}
			segs.push(seg);
		}
{{else if eq .Type "float32"}}
		if (this.{{.NameNative}} || Number.isNaN(this.{{.NameNative}})) {
			if (this.{{.NameNative}} > 3.4028234663852886E38 || this.{{.NameNative}} < -3.4028234663852886E38)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds 32-bit range';
			var bytes = new Uint8Array(5);
			bytes[0] = {{.Index}};
			new DataView(bytes.buffer).setFloat32(1, this.{{.NameNative}});
			segs.push(bytes);
		}
{{else if eq .Type "float64"}}
		if (this.{{.NameNative}} || Number.isNaN(this.{{.NameNative}})) {
			var bytes = new Uint8Array(9);
			bytes[0] = {{.Index}};
			new DataView(bytes.buffer).setFloat64(1, this.{{.NameNative}});
			segs.push(bytes);
		}
{{else if eq .Type "timestamp"}}
		if ((this.{{.NameNative}} && this.{{.NameNative}}.getTime()) || this.{{.NameNative}}_ns) {
			var ms = this.{{.NameNative}} ? this.{{.NameNative}}.getTime() : 0;
			if (ms < -Number.MAX_SAFE_INTEGER || ms > Number.MAX_SAFE_INTEGER)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} millisecond value exceeds Number.MAX_SAFE_INTEGER';
			var s = ms / 1E3;

			var ns = this.{{.NameNative}}_ns || 0;
			if (ns < 0 || ns >= 1E6)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}}_ns not in range (0, 1ms>';
			var msf = ms % 1E3;
			if (ms < 0 && msf) {
				s--
				msf = 1E3 + msf;
			}
			ns += msf * 1E6;

			if (s > 0xffffffff || s < 0) {
				var bytes = new Uint8Array(13);
				bytes[0] = {{.Index}} | 128;
				var view = new DataView(bytes.buffer);
				view.setUint32(9, ns);
				if (s > 0) {
					view.setUint32(1, s / 0x100000000);
					view.setUint32(5, s);
				} else {
					s = -s;
					view.setUint32(1, s / 0x100000000);
					view.setUint32(5, s);
					var carry = 1;
					for (var j = 8; j > 0; j--) {
						var b = (bytes[j] ^ 255) + carry;
						bytes[j] = b & 255;
						carry = b >> 8;
					}
				}
				segs.push(bytes);
			} else {
				var bytes = new Uint8Array(9);
				bytes[0] = {{.Index}};
				var view = new DataView(bytes.buffer);
				view.setUint32(1, s);
				view.setUint32(5, ns);
				segs.push(bytes);
			}
		}
{{else if eq .Type "text"}}
 {{- if .TypeList}}
		if (this.{{.NameNative}} && this.{{.NameNative}}.length) {
			var a = this.{{.NameNative}};
			if (a.length > colferListMax)
				throw 'colfer: {{.String}} length exceeds colferListMax';
			var seg = [{{.Index}}];
			encodeVarint(seg, a.length);
			segs.push(seg);
			for (var i = 0; i < a.length; i++) {
				var s = a[i];
				if (s == null) {
					s = "";
					a[i] = s;
				}
				var utf = encodeUTF8(s);
				seg = [];
				encodeVarint(seg, utf.length);
				segs.push(seg);
				segs.push(utf)
			}
		}
 {{- else}}
		if (this.{{.NameNative}}) {
			var utf = encodeUTF8(this.{{.NameNative}});
			var seg = [{{.Index}}];
			encodeVarint(seg, utf.length);
			segs.push(seg);
			segs.push(utf)
		}
 {{- end}}
{{else if eq .Type "binary"}}
 {{- if .TypeList}}
		if (this.{{.NameNative}} && this.{{.NameNative}}.length) {
			var a = this.{{.NameNative}};
			if (a.length > colferListMax)
				throw 'colfer: {{.String}} length exceeds colferListMax';
			var seg = [{{.Index}}];
			encodeVarint(seg, a.length);
			segs.push(seg);
			for (var i = 0; i < a.length; i++) {
				var b = a[i];
				if (b == null) {
					b = new Uint8Array(0);
					a[i] = b;
				}
				seg = [];
				encodeVarint(seg, b.length);
				segs.push(seg);
				segs.push(b)
			}
		}
 {{- else}}
		if (this.{{.NameNative}} && this.{{.NameNative}}.length) {
			var seg = [{{.Index}}];
			encodeVarint(seg, this.{{.NameNative}}.length);
			segs.push(seg);
			segs.push(this.{{.NameNative}});
		}
 {{- end}}
{{else if .TypeList}}
		if (this.{{.NameNative}} && this.{{.NameNative}}.length) {
			var a = this.{{.NameNative}};
			if (a.length > colferListMax)
				throw 'colfer: {{.String}} length exceeds colferListMax';
			var seg = [{{.Index}}];
			encodeVarint(seg, a.length);
			segs.push(seg);
			for (var i = 0; i < a.length; i++) {
				var v = a[i];
				if (v == null) {
					v = new {{.TypeRef.Pkg.NameNative}}.{{.TypeRef.NameTitle}}();
					a[i] = v;
				}
				segs.push(v.marshal());
			};
		}
{{else}}
		if (this.{{.NameNative}}) {
			segs.push([{{.Index}}]);
			segs.push(this.{{.NameNative}}.marshal());
		}
{{end}}{{end}}
		var size = 1;
		segs.forEach(function(seg) {
			size += seg.length;
		});
		if (size > colferSizeMax)
			throw 'colfer: {{.String}} serial size ' + size + ' exceeds ' + colferListMax + ' bytes';

		var bytes = new Uint8Array(size);
		var i = 0;
		segs.forEach(function(seg) {
			bytes.set(seg, i);
			i += seg.length;
		});
		bytes[i] = 127;
		return bytes;
	}`

const ecmaUnmarshal = `
	// Deserializes the object from an Uint8Array and returns the number of bytes read.
	this.{{.NameTitle}}.prototype.unmarshal = function(data) {
		if (!data || ! data.length) throw EOF;
		var header = data[0];
		var i = 1;
		var readHeader = function() {
			if (i >= data.length) throw EOF;
			header = data[i++];
		}

		var readVarint = function() {
			var pos = 0, result = 0;
			while (pos != 8) {
				var c = data[i+pos];
				result += (c & 127) * Math.pow(128, pos);
				++pos;
				if (c < 128) {
					i += pos;
					if (result > Number.MAX_SAFE_INTEGER) break;
					return result;
				}
				if (pos == data.length) throw EOF;
			}
			return -1;
		}
{{range .Fields}}{{if eq .Type "bool"}}
		if (header == {{.Index}}) {
			this.{{.NameNative}} = true;
			readHeader();
		}
{{else if eq .Type "uint8"}}
		if (header == {{.Index}}) {
			if (i + 1 >= data.length) throw EOF;
			this.{{.NameNative}} = data[i++];
			header = data[i++];
		}
{{else if eq .Type "uint16"}}
		if (header == {{.Index}}) {
			if (i + 2 >= data.length) throw EOF;
			this.{{.NameNative}} = (data[i++] << 8) | data[i++];
			header = data[i++];
		} else if (header == ({{.Index}} | 128)) {
			if (i + 1 >= data.length) throw EOF;
			this.{{.NameNative}} = data[i++];
			header = data[i++];
		}
{{else if eq .Type "uint32"}}
		if (header == {{.Index}}) {
			var x = readVarint();
			if (x < 0) throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
			this.{{.NameNative}} = x;
			readHeader();
		} else if (header == ({{.Index}} | 128)) {
			if (i + 4 > data.length) throw EOF;
			this.{{.NameNative}} = new DataView(data.buffer).getUint32(i);
			i += 4;
			readHeader();
		}
{{else if eq .Type "uint64"}}
		if (header == {{.Index}}) {
			var x = readVarint();
			if (x < 0) throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
			this.{{.NameNative}} = x;
			readHeader();
		} else if (header == ({{.Index}} | 128)) {
			if (i + 8 > data.length) throw EOF;
			var view = new DataView(data.buffer);
			var x = view.getUint32(i) * 0x100000000;
			x += view.getUint32(i + 4);
			if (x > Number.MAX_SAFE_INTEGER)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
			this.{{.NameNative}} = x;
			i += 8;
			readHeader();
		}
{{else if eq .Type "int32"}}
		if (header == {{.Index}}) {
			var x = readVarint();
			if (x < 0) throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
			this.{{.NameNative}} = x;
			readHeader();
		} else if (header == ({{.Index}} | 128)) {
			var x = readVarint();
			if (x < 0) throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
			this.{{.NameNative}} = -1 * x;
			readHeader();
		}
{{else if eq .Type "int64"}}
		if (header == {{.Index}}) {
			var x = readVarint();
			if (x < 0) throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
			this.{{.NameNative}} = x;
			readHeader();
		} else if (header == ({{.Index}} | 128)) {
			var x = readVarint();
			if (x < 0) throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} exceeds Number.MAX_SAFE_INTEGER';
			this.{{.NameNative}} = -1 * x;
			readHeader();
		}
{{else if eq .Type "float32"}}
		if (header == {{.Index}}) {
			if (i + 4 > data.length) throw EOF;
			this.{{.NameNative}} = new DataView(data.buffer).getFloat32(i);
			i += 4;
			readHeader();
		}
{{else if eq .Type "float64"}}
		if (header == {{.Index}}) {
			if (i + 8 > data.length) throw EOF;
			this.{{.NameNative}} = new DataView(data.buffer).getFloat64(i);
			i += 8;
			readHeader();
		}
{{else if eq .Type "timestamp"}}
		if (header == {{.Index}}) {
			if (i + 8 > data.length) throw EOF;
			var view = new DataView(data.buffer);
			var ms = view.getUint32(i) * 1000;
			var ns = view.getUint32(i + 4);
			ms += ns / 1E6;
			ns %= 1E6;
			if (ms > Number.MAX_SAFE_INTEGER)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} millisecond value exceeds Number.MAX_SAFE_INTEGER';
			i += 8;
			this.{{.NameNative}} = new Date();
			this.{{.NameNative}}.setTime(ms);
			this.{{.NameNative}}_ns = ns;
			readHeader();
		} else if (header == ({{.Index}} | 128)) {
			if (i + 12 > data.length) throw EOF;

			var int64 = new Uint8Array(data.subarray(i, i + 8));
			if (int64[0] > 127) {	// two's complement
				var carry = 1;
				for (var j = 7; j >= 0; j--) {
					var b = (int64[j] ^ 255) + carry;
					int64[j] = b & 255;
					carry = b >> 8;
				}
			}
			if (int64[0] != 0 || int64[1] > 31)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} second value exceeds Number.MAX_SAFE_INTEGER';
			var view = new DataView(int64.buffer);
			var s = (view.getUint32(0) * 0x100000000) + view.getUint32(4);
			if (data[i] > 127) s = -s;

			var ns = new DataView(data.buffer).getUint32(i + 8);
			var ms = (s * 1E3);
			if (Math.abs(ms) > Number.MAX_SAFE_INTEGER)
				throw 'colfer: {{.Struct.Pkg.NameNative}}/{{.Struct.NameTitle}} field {{.NameNative}} millisecond value exceeds Number.MAX_SAFE_INTEGER';
			var msa = Math.floor(ns / 1E6);
			if (msa > 0) {
				if (s < 0) ms = (ms + 1000) - (1000 - msa);
				else ms += msa;
			}
			this.{{.NameNative}} = new Date();
			this.{{.NameNative}}.setTime(ms);
			this.{{.NameNative}}_ns = ns % 1E6;

			i += 12;
			readHeader();
		}
{{else if eq .Type "text"}}
		if (header == {{.Index}}) {
 {{- if .TypeList}}
			var length = readVarint();
			if (length < 0)
				throw 'colfer: {{.String}} length exceeds Number.MAX_SAFE_INTEGER';
			if (length > colferListMax)
				throw 'colfer: {{.String}} length ' + length + ' exceeds ' + colferListMax + ' elements';
			while (--length >= 0) {
				var size = readVarint();
				if (size < 0)
					throw 'colfer: {{.String}} element ' + this.{{.NameNative}}.length + ' size exceeds Number.MAX_SAFE_INTEGER';
				else if (size > colferSizeMax)
					throw 'colfer: {{.String}} element ' + this.{{.NameNative}}.length + ' size ' + size + ' exceeds ' + colferSizeMax + ' UTF-8 bytes';
				var to = i + size;
				if (to > data.length) throw EOF;
				this.{{.NameNative}}.push(decodeUTF8(data.subarray(i, to)));
				i = to;
			}
 {{- else}}
			var size = readVarint();
			if (size < 0)
				throw 'colfer: {{.String}} size exceeds Number.MAX_SAFE_INTEGER';
			else if (size > colferSizeMax)
				throw 'colfer: {{.String}} size ' + size + ' exceeds ' + colferSizeMax + ' UTF-8 bytes';
			var to = i + size;
			if (to > data.length) throw EOF;
			this.{{.NameNative}} = decodeUTF8(data.subarray(i, to));
			i = to;
 {{- end}}
			readHeader();
		}
{{else if eq .Type "binary"}}
		if (header == {{.Index}}) {
 {{- if .TypeList}}
			var length = readVarint();
			if (length < 0)
				throw 'colfer: {{.String}} length exceeds Number.MAX_SAFE_INTEGER';
			if (length > colferListMax)
				throw 'colfer: {{.String}} length ' + length + ' exceeds ' + colferListMax + ' elements';
			while (--length >= 0) {
				var size = readVarint();
				if (size < 0)
					throw 'colfer: {{.String}} element ' + this.{{.NameNative}}.length + ' size exceeds Number.MAX_SAFE_INTEGER';
				else if (size > colferSizeMax)
					throw 'colfer: {{.String}} element ' + this.{{.NameNative}}.length + ' size ' + size + ' exceeds ' + colferSizeMax + ' UTF-8 bytes';
				var to = i + size;
				if (to > data.length) throw EOF;
				this.{{.NameNative}}.push(data.subarray(i, to));
				i = to;
			}
 {{- else}}
			var size = readVarint();
			if (size < 0)
				throw 'colfer: {{.String}} size exceeds Number.MAX_SAFE_INTEGER';
			else if (size > colferSizeMax)
				throw 'colfer: {{.String}} size ' + size + ' exceeds ' + colferSizeMax + ' bytes';
			var to = i + size;
			if (to > data.length) throw EOF;
			this.{{.NameNative}} = data.subarray(i, to);
			i = to;
 {{- end}}
			readHeader();
		}
{{else if .TypeList}}
		if (header == {{.Index}}) {
			var length = readVarint();
			if (length < 0)
				throw 'colfer: {{.String}} length exceeds Number.MAX_SAFE_INTEGER';
			if (length > colferListMax)
				throw 'colfer: {{.String}} length ' + length + ' exceeds ' + colferListMax + ' elements';
			while (--length >= 0) {
				var o = new {{.TypeRef.Pkg.NameNative}}.{{.TypeRef.NameTitle}}();
				i += o.unmarshal(data.subarray(i));
				this.{{.NameNative}}.push(o);
			}
			readHeader();
		}
{{else}}
		if (header == {{.Index}}) {
			var o = new {{.TypeRef.Pkg.NameNative}}.{{.TypeRef.NameTitle}}();
			i += o.unmarshal(data.subarray(i));
			this.{{.NameNative}} = o;
			readHeader();
		}
{{end}}{{end}}
		if (header != 127) throw 'colfer: unknown header at byte ' + (i - 1);
		if (i > colferSizeMax)
			throw 'colfer: {{.String}} serial size ' + size + ' exceeds ' + colferSizeMax + ' bytes';
		return i;
	}`
