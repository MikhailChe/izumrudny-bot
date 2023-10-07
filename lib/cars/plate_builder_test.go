package cars

import "testing"

func TestNextCharacterType(t *testing.T) {
	type args struct {
		current string
	}
	tests := []struct {
		name     string
		args     args
		wantNext characterType
	}{
		{
			name:     "empty",
			args:     args{current: ""},
			wantNext: Number | LatinoCyrillic,
		},
		{
			name:     "X",
			args:     args{current: "X"},
			wantNext: Number,
		},
		{
			name:     "X7",
			args:     args{current: "X7"},
			wantNext: Number,
		},
		{
			name:     "X70",
			args:     args{current: "X70"},
			wantNext: Number,
		},
		{
			name:     "X703",
			args:     args{current: "X703"},
			wantNext: LatinoCyrillic,
		},
		{
			name:     "X703B",
			args:     args{current: "X703B"},
			wantNext: LatinoCyrillic,
		},
		{
			name:     "X703BX",
			args:     args{current: "X703BX"},
			wantNext: Number,
		},
		{
			name:     "X703BX9",
			args:     args{current: "X703BX9"},
			wantNext: Number,
		},
		{
			name:     "X703BX96",
			args:     args{current: "X703BX96"},
			wantNext: None | Number,
		},
		{
			name:     "X703BX196",
			args:     args{current: "X703BX196"},
			wantNext: None,
		},
		{
			name:     "6",
			args:     args{current: "6"},
			wantNext: Number,
		},
		{
			name:     "68",
			args:     args{current: "68"},
			wantNext: Number,
		},
		{
			name:     "682",
			args:     args{current: "682"},
			wantNext: Number,
		},
		{
			name:     "6822",
			args:     args{current: "6822"},
			wantNext: LatinoCyrillic,
		},
		{
			name:     "6822B",
			args:     args{current: "6822B"},
			wantNext: LatinoCyrillic,
		},
		{
			name:     "6822BA",
			args:     args{current: "6822BA"},
			wantNext: Number,
		},
		{
			name:     "6822BA9",
			args:     args{current: "6822BA9"},
			wantNext: Number,
		},
		{
			name:     "6822BA96",
			args:     args{current: "6822BA96"},
			wantNext: None | Number,
		},
		{
			name:     "6822BA196",
			args:     args{current: "6822BA196"},
			wantNext: None,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotNext := NextCharacterType(tt.args.current); gotNext != tt.wantNext {
				t.Errorf("NextCharacterType() = %v, want %v", gotNext, tt.wantNext)
			}
		})
	}
}
