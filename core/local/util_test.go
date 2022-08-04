package local

//TODO: translate this test to the new paradigm
/*
func TestProcessStages(t *testing.T) {
	type checkpoint struct {
		D    time.Duration
		Keep bool
		VUs  null.Int
	}
	testdata := map[string]struct {
		Start       int64
		Stages      []lib.Stage
		Checkpoints []checkpoint
	}{
		"none": {
			0,
			[]lib.Stage{},
			[]checkpoint{
				{0 * time.Second, false, null.NewInt(0, false)},
				{10 * time.Second, false, null.NewInt(0, false)},
				{24 * time.Hour, false, null.NewInt(0, false)},
			},
		},
		"one": {
			0,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(10 * time.Second)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.NewInt(0, false)},
				{1 * time.Second, true, null.NewInt(0, false)},
				{10 * time.Second, true, null.NewInt(0, false)},
				{11 * time.Second, false, null.NewInt(0, false)},
			},
		},
		"one/start": {
			5,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(10 * time.Second)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.NewInt(5, false)},
				{1 * time.Second, true, null.NewInt(5, false)},
				{10 * time.Second, true, null.NewInt(5, false)},
				{11 * time.Second, false, null.NewInt(5, false)},
			},
		},
		"one/targeted": {
			0,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(100)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.IntFrom(0)},
				{1 * time.Second, true, null.IntFrom(10)},
				{2 * time.Second, true, null.IntFrom(20)},
				{3 * time.Second, true, null.IntFrom(30)},
				{4 * time.Second, true, null.IntFrom(40)},
				{5 * time.Second, true, null.IntFrom(50)},
				{6 * time.Second, true, null.IntFrom(60)},
				{7 * time.Second, true, null.IntFrom(70)},
				{8 * time.Second, true, null.IntFrom(80)},
				{9 * time.Second, true, null.IntFrom(90)},
				{10 * time.Second, true, null.IntFrom(100)},
				{11 * time.Second, false, null.IntFrom(100)},
			},
		},
		"one/targeted/start": {
			50,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(100)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.IntFrom(50)},
				{1 * time.Second, true, null.IntFrom(55)},
				{2 * time.Second, true, null.IntFrom(60)},
				{3 * time.Second, true, null.IntFrom(65)},
				{4 * time.Second, true, null.IntFrom(70)},
				{5 * time.Second, true, null.IntFrom(75)},
				{6 * time.Second, true, null.IntFrom(80)},
				{7 * time.Second, true, null.IntFrom(85)},
				{8 * time.Second, true, null.IntFrom(90)},
				{9 * time.Second, true, null.IntFrom(95)},
				{10 * time.Second, true, null.IntFrom(100)},
				{11 * time.Second, false, null.IntFrom(100)},
			},
		},
		"two": {
			0,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.NewInt(0, false)},
				{1 * time.Second, true, null.NewInt(0, false)},
				{11 * time.Second, false, null.NewInt(0, false)},
			},
		},
		"two/start": {
			5,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.NewInt(5, false)},
				{1 * time.Second, true, null.NewInt(5, false)},
				{11 * time.Second, false, null.NewInt(5, false)},
			},
		},
		"two/targeted": {
			0,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(100)},
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(0)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.IntFrom(0)},
				{1 * time.Second, true, null.IntFrom(20)},
				{2 * time.Second, true, null.IntFrom(40)},
				{3 * time.Second, true, null.IntFrom(60)},
				{4 * time.Second, true, null.IntFrom(80)},
				{5 * time.Second, true, null.IntFrom(100)},
				{6 * time.Second, true, null.IntFrom(80)},
				{7 * time.Second, true, null.IntFrom(60)},
				{8 * time.Second, true, null.IntFrom(40)},
				{9 * time.Second, true, null.IntFrom(20)},
				{10 * time.Second, true, null.IntFrom(0)},
				{11 * time.Second, false, null.IntFrom(0)},
			},
		},
		"three": {
			0,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{Duration: types.NullDurationFrom(10 * time.Second)},
				{Duration: types.NullDurationFrom(15 * time.Second)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.NewInt(0, false)},
				{1 * time.Second, true, null.NewInt(0, false)},
				{15 * time.Second, true, null.NewInt(0, false)},
				{30 * time.Second, true, null.NewInt(0, false)},
				{31 * time.Second, false, null.NewInt(0, false)},
			},
		},
		"three/targeted": {
			0,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(50)},
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(100)},
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(0)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.IntFrom(0)},
				{1 * time.Second, true, null.IntFrom(10)},
				{2 * time.Second, true, null.IntFrom(20)},
				{3 * time.Second, true, null.IntFrom(30)},
				{4 * time.Second, true, null.IntFrom(40)},
				{5 * time.Second, true, null.IntFrom(50)},
				{6 * time.Second, true, null.IntFrom(60)},
				{7 * time.Second, true, null.IntFrom(70)},
				{8 * time.Second, true, null.IntFrom(80)},
				{9 * time.Second, true, null.IntFrom(90)},
				{10 * time.Second, true, null.IntFrom(100)},
				{11 * time.Second, true, null.IntFrom(80)},
				{12 * time.Second, true, null.IntFrom(60)},
				{13 * time.Second, true, null.IntFrom(40)},
				{14 * time.Second, true, null.IntFrom(20)},
				{15 * time.Second, true, null.IntFrom(0)},
				{16 * time.Second, false, null.IntFrom(0)},
			},
		},
		"mix": {
			0,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(20)},
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(10)},
				{Duration: types.NullDurationFrom(2 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(20)},
				{Duration: types.NullDurationFrom(2 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(10)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.IntFrom(0)},

				{1 * time.Second, true, null.IntFrom(4)},
				{2 * time.Second, true, null.IntFrom(8)},
				{3 * time.Second, true, null.IntFrom(12)},
				{4 * time.Second, true, null.IntFrom(16)},
				{5 * time.Second, true, null.IntFrom(20)},

				{6 * time.Second, true, null.IntFrom(18)},
				{7 * time.Second, true, null.IntFrom(16)},
				{8 * time.Second, true, null.IntFrom(14)},
				{9 * time.Second, true, null.IntFrom(12)},
				{10 * time.Second, true, null.IntFrom(10)},

				{11 * time.Second, true, null.IntFrom(10)},
				{12 * time.Second, true, null.IntFrom(10)},

				{13 * time.Second, true, null.IntFrom(12)},
				{14 * time.Second, true, null.IntFrom(14)},
				{15 * time.Second, true, null.IntFrom(16)},
				{16 * time.Second, true, null.IntFrom(18)},
				{17 * time.Second, true, null.IntFrom(20)},

				{18 * time.Second, true, null.IntFrom(20)},
				{19 * time.Second, true, null.IntFrom(20)},

				{20 * time.Second, true, null.IntFrom(18)},
				{21 * time.Second, true, null.IntFrom(16)},
				{22 * time.Second, true, null.IntFrom(14)},
				{23 * time.Second, true, null.IntFrom(12)},
				{24 * time.Second, true, null.IntFrom(10)},
			},
		},
		"mix/start": {
			5,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(10)},
			},
			[]checkpoint{
				{0 * time.Second, true, null.NewInt(5, false)},

				{1 * time.Second, true, null.NewInt(5, false)},
				{2 * time.Second, true, null.NewInt(5, false)},
				{3 * time.Second, true, null.NewInt(5, false)},
				{4 * time.Second, true, null.NewInt(5, false)},
				{5 * time.Second, true, null.NewInt(5, false)},

				{6 * time.Second, true, null.NewInt(6, true)},
				{7 * time.Second, true, null.NewInt(7, true)},
				{8 * time.Second, true, null.NewInt(8, true)},
				{9 * time.Second, true, null.NewInt(9, true)},
				{10 * time.Second, true, null.NewInt(10, true)},
			},
		},
		"infinite": {
			0,
			[]lib.Stage{{}},
			[]checkpoint{
				{0 * time.Second, true, null.NewInt(0, false)},
				{1 * time.Minute, true, null.NewInt(0, false)},
				{1 * time.Hour, true, null.NewInt(0, false)},
				{24 * time.Hour, true, null.NewInt(0, false)},
				{365 * 24 * time.Hour, true, null.NewInt(0, false)},
			},
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			for _, ckp := range data.Checkpoints {
				t.Run(ckp.D.String(), func(t *testing.T) {
					vus, keepRunning := ProcessStages(data.Start, data.Stages, ckp.D)
					assert.Equal(t, ckp.VUs, vus)
					assert.Equal(t, ckp.Keep, keepRunning)
				})
			}
		})
	}
}
*/
