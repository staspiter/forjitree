package forjitree

import (
	"reflect"
	"testing"
)

func TestTokenizePath(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want []pathToken
	}{
		{
			name: "Root path",
			args: args{
				path: "/",
			},
			want: []pathToken{
				{
					Kind: PathTokenKindRoot,
				},
				{
					Kind: PathTokenKindThis,
				},
			},
		},

		{
			name: "Root path with sub",
			args: args{
				path: "/sub",
			},
			want: []pathToken{
				{
					Kind: PathTokenKindRoot,
				},
				{
					Kind: PathTokenKindSub,
					Key:  "sub",
				},
			},
		},

		{
			name: "Param",
			args: args{
				path: "sub[key=value]",
			},
			want: []pathToken{
				{
					Kind: PathTokenKindSub,
					Key:  "sub",
				},
				{
					Kind: PathTokenKindParams,
					Params: []pathTokenParam{
						{
							"key",
							"value",
							ParamTypeEquals,
							nil,
						},
					},
				},
			},
		},

		{
			name: "Enclosed paths in params support",
			args: args{
				path: "sub[key/subkey=value]",
			},
			want: []pathToken{
				{
					Kind: PathTokenKindSub,
					Key:  "sub",
				},
				{
					Kind: PathTokenKindParams,
					Params: []pathTokenParam{
						{
							"key/subkey",
							"value",
							ParamTypeEquals,
							nil,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TokenizePath(tt.args.path); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TokenizePath() = %v, want %v", got, tt.want)
			}
		})
	}
}
