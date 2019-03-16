package mobi

import (
	"bytes"
	"testing"
)

// Test performance and correctness of lz77 compression to allow us to speed it up
func TestCorrectness(t *testing.T) {
	input := testData()
	if len(input) > 4096 {
		t.Error("I've created too-long input")
		t.FailNow()
	}
	expected := palmDocLZ77Pack(input)
	actual := fastpalmDocLZ77Pack(input)
	if !bytes.Equal(expected, actual) {
		t.Error("Difference in compression")
		if len(expected) != len(actual) {
			t.Errorf("Difference in output length: expected %d, got %d", len(expected), len(actual))
		}
		sz := len(actual)
		if len(expected) < len(actual) {
			sz = len(expected)
		}
		for i := 0; i < sz; i++ {
			if expected[i] != actual[i] {
				t.Errorf("First difference in output at index %d", i)
				break
			}
		}
	}
}

var devnull []byte

func BenchmarkLZ77(b *testing.B) {
	input := testData()
	var res []byte
	for n := 0; n < b.N; n++ {
		res = palmDocLZ77Pack(input)
	}
	devnull = res
}

func BenchmarkFastLZ77(b *testing.B) {
	input := testData()
	var res []byte
	for n := 0; n < b.N; n++ {
		res = fastpalmDocLZ77Pack(input)
	}
	devnull = res
}

func testData() []byte {
	data := []byte(lipsum)
	// Adding a zero byte to indicate that we don't have a tail
	data = append(data, 0)
	return data
}

// some html-formatted test data to simulate use case
const lipsum = `<div id="lipsum">
<p>
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Cras fermentum molestie urna, in mattis orci placerat non. Duis bibendum risus ac erat molestie, tincidunt sollicitudin metus malesuada. Maecenas rhoncus tincidunt ultricies. Cras vel vehicula diam. Ut laoreet tellus eget pellentesque ullamcorper. Etiam rutrum fringilla nulla ut pulvinar. Donec non tortor et enim mattis porta vitae in tellus. Duis convallis facilisis odio, ac ornare velit posuere in. Donec sapien odio, vehicula ac purus et, imperdiet aliquam quam. Aliquam a pellentesque leo. Suspendisse feugiat mauris ac finibus pharetra. Fusce vitae ex nec ante faucibus ullamcorper. Etiam malesuada erat ut dictum ultrices. Sed eu massa nunc.
</p>
<p>
Aliquam id mollis ante. Aenean eleifend nisi libero, in dapibus mi vulputate sit amet. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae; Nam vel lectus tempus, congue est ac, lobortis lectus. Sed ullamcorper vitae purus non condimentum. In vel tellus nisi. Phasellus id turpis vel turpis elementum sagittis. Etiam a interdum risus, eget rutrum arcu. Aenean aliquam arcu et lorem auctor, ut hendrerit nisi viverra. Nunc scelerisque, ante at tempor feugiat, nisl nunc varius felis, quis congue dolor odio sed lorem. Duis id fringilla dolor.
</p>
<p>
Donec non odio tincidunt, dapibus urna quis, cursus ex. Curabitur leo libero, varius at condimentum sit amet, vehicula eget sapien. Fusce tincidunt pulvinar egestas. Phasellus non massa sed nunc sodales placerat. Duis id aliquam ex. Donec ac commodo ante. Sed venenatis, orci ut sagittis imperdiet, lacus nulla commodo odio, nec molestie massa erat ac urna. Donec at nunc purus. Donec eget sollicitudin ex. Duis tristique et lorem ac aliquet.
</p>
<p>
Sed sagittis tincidunt ligula non ultricies. Integer egestas, odio ac porta ultrices, est felis hendrerit est, vitae posuere eros lorem eu ipsum. Vivamus non lacinia risus, a sagittis turpis. Vivamus sit amet aliquam justo. Vestibulum dictum diam sit amet faucibus sollicitudin. Phasellus eu mollis diam. Fusce id bibendum ligula. Curabitur maximus nunc non dolor tincidunt convallis. Integer eleifend faucibus fringilla.
</p>
<p>
Integer feugiat diam turpis, non mollis purus ornare ac. Nunc nec blandit odio. Phasellus porttitor, lacus eu aliquet commodo, quam leo vulputate turpis, sit amet condimentum metus urna eget turpis. Nunc id viverra ex. Etiam ultrices nisl nisi, sit amet placerat diam imperdiet in. Nunc nibh felis, fermentum non nulla sed, ultricies malesuada orci. Curabitur vel justo non turpis vehicula feugiat. Curabitur placerat ac lacus ut rhoncus. Nulla convallis bibendum leo, vel hendrerit dui eleifend ut. Aenean eu enim lacus. Vestibulum sed diam ut nulla imperdiet mattis. Curabitur nec ante sit amet urna egestas ullamcorper.
</p>
<p>
Aliquam et eros ut nibh convallis commodo ac vitae nisl. Aenean sit amet fermentum orci. Morbi finibus bibendum nisl ut elementum. Curabitur ornare ante id malesuada ullamcorper. Mauris varius bibendum tortor ut mollis. Vestibulum efficitur massa id elit dictum cursus. Phasellus ligula nulla, finibus molestie viverra ut, eleifend ut velit. Quisque pharetra vitae nisi fermentum laoreet. In tincidunt lacus nec ex lobortis dignissim.
</p>
<p>
Curabitur vel justo consectetur risus euismod aliquam. Praesent rhoncus aliquet felis, at ullamcorper massa sagittis sed. Vivamus vel est in dui pretium sodales quis sit amet lorem. Vivamus tempor mattis rutrum. Curabitur accumsan, mauris vitae imperdiet dapibus, quam lectus facilisis urna nullam.
</p></div>`
