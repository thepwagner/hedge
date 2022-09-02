package debian_test

// func TestWriteReleaseFile(t *testing.T) {
// 	rel := debian.Release{
// 		Origin:   "Debian",
// 		Codename: "bullseye",
// 	}
// 	pkgs := map[debian.Architecture][]debian.Package{
// 		"amd64": {
// 			{Package: "foo", Version: "1.0"},
// 		},
// 	}

// 	var buf bytes.Buffer
// 	err := debian.WriteReleaseFile(context.Background(), rel, pkgs, &buf)
// 	require.NoError(t, err)

// 	assert.Equal(t, `Codename: bullseye
// Origin: Debian
// MD5Sum:
//   496dc54a775e228596fea4d6510532c7 26 main/binary-amd64/Packages
//   d84f4fd0a94f0f586cb63691b38c4589 50 main/binary-amd64/Packages.gz
// SHA256:
//   58d78f21ebcded90d2e20cf81eb98622b95f19e577893a2acc109df9b5e6128b 26 main/binary-amd64/Packages
//   fe01b3ec9920bbd6fc4b8ed887711edd07a7b0a06605f9cca91f2f147f6ee1eb 50 main/binary-amd64/Packages.gz
// `, buf.String())
// }
