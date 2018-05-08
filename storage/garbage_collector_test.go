package storage_test

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/bosh-bootloader/fakes"
	"github.com/cloudfoundry/bosh-bootloader/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("garbage collector", func() {
	var (
		gc     storage.GarbageCollector
		printer *fakes.Logger
		fileIO *fakes.FileIO
	)

	BeforeEach(func() {
		fileIO = &fakes.FileIO{}
		printer = &fakes.Logger{}
		gc = storage.NewGarbageCollector(fileIO, printer)
	})

	Describe("remove", func() {
		It("removes the bbl-state.json file", func() {
			err := gc.Remove("some-dir")
			Expect(err).NotTo(HaveOccurred())

			Expect(fileIO.RemoveCall.Receives[0].Name).To(Equal(filepath.Join("some-dir", "bbl-state.json")))
		})

		It("removes bosh *-env scripts", func() {
			createDirector := filepath.Join("some-dir", "create-director.sh")
			createJumpbox := filepath.Join("some-dir", "create-jumpbox.sh")
			deleteDirector := filepath.Join("some-dir", "delete-director.sh")
			deleteJumpbox := filepath.Join("some-dir", "delete-jumpbox.sh")

			err := gc.Remove("some-dir")
			Expect(err).NotTo(HaveOccurred())

			Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{Name: createDirector}))
			Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{Name: deleteDirector}))
			Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{Name: deleteJumpbox}))
			Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{Name: createJumpbox}))
		})

		DescribeTable("removing bbl-created directories",
			func(directory string, expectToBeDeleted bool) {
				err := gc.Remove("some-dir")
				Expect(err).NotTo(HaveOccurred())

				if expectToBeDeleted {
					Expect(fileIO.RemoveAllCall.Receives).To(ContainElement(fakes.RemoveAllReceive{
						Path: filepath.Join("some-dir", directory),
					}))
				} else {
					Expect(fileIO.RemoveAllCall.Receives).NotTo(ContainElement(fakes.RemoveAllReceive{
						Path: filepath.Join("some-dir", directory),
					}))
				}
			},
			Entry(".terraform", ".terraform", true),
			Entry("bosh-deployment", "bosh-deployment", true),
			Entry("jumpbox-deployment", "jumpbox-deployment", true),
			Entry("bbl-ops-files", "bbl-ops-files", true),
			Entry("non-bbl directory", "foo", false),
		)

		Describe("cloud-config", func() {
			var (
				cloudConfigBase string
				cloudConfigOps  string
			)
			BeforeEach(func() {
				cloudConfigBase = filepath.Join("some-dir", "cloud-config", "cloud-config.yml")
				cloudConfigOps = filepath.Join("some-dir", "cloud-config", "ops.yml")
			})

			It("removes the ops file, base file, and directory", func() {
				err := gc.Remove("some-dir")
				Expect(err).NotTo(HaveOccurred())

				Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{Name: cloudConfigBase}))
				Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{Name: cloudConfigOps}))
				Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{
					Name: filepath.Join("some-dir", "cloud-config"),
				}))
			})

			Context("when the cloud-config directory contains user managed files", func() {
				BeforeEach(func() {
					fileIO.ReadDirCall.Fake = func(dirname string) ([]os.FileInfo, error) {
						if dirname == "some-dir/cloud-config" {
							return []os.FileInfo{fakes.FileInfo{FileName: "user-managed-ops.yml"}}, nil
						}
						return []os.FileInfo{}, nil
					}
				})

				It("prints a warning about what files are left", func() {
					err := gc.Remove("some-dir")
					Expect(err).NotTo(HaveOccurred())
					Expect(printer.PrintfCall.Messages).To(ConsistOf(filepath.Join("some-dir", "cloud-config", "user-managed-ops.yml")))
					Expect(printer.PrintfCall.CallCount).To(Equal(1))
				})
			})
		})

		Describe("vars", func() {
			Context("when the vars directory contains only bbl files", func() {
				BeforeEach(func() {
					fileIO.ReadDirCall.Returns.FileInfos = []os.FileInfo{
						fakes.FileInfo{FileName: "bbl.tfvars"},
						fakes.FileInfo{FileName: "bosh-state.json"},
						fakes.FileInfo{FileName: "cloud-config-vars.yml"},
						fakes.FileInfo{FileName: "director-vars-file.yml"},
						fakes.FileInfo{FileName: "director-vars-store.yml"},
						fakes.FileInfo{FileName: "jumpbox-state.json"},
						fakes.FileInfo{FileName: "jumpbox-vars-file.yml"},
						fakes.FileInfo{FileName: "jumpbox-vars-store.yml"},
						fakes.FileInfo{FileName: "terraform.tfstate"},
						fakes.FileInfo{FileName: "terraform.tfstate.backup"},
					}
				})

				It("removes the directory", func() {
					err := gc.Remove("some-dir")
					Expect(err).NotTo(HaveOccurred())

					Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{
						Name: filepath.Join("some-dir", "vars", "bbl.tfvars"),
					}))
					Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{
						Name: filepath.Join("some-dir", "vars"),
					}))
				})
			})

			Context("when the vars directory contains user managed files", func() {
				BeforeEach(func() {
					fileIO.ReadDirCall.Fake = func(dirname string) ([]os.FileInfo, error) {
						if dirname == "some-dir/vars" {
							return []os.FileInfo{
								fakes.FileInfo{FileName: "user-managed-file"},
								fakes.FileInfo{FileName: "terraform.tfstate.backup"},
							}, nil
						}
						return []os.FileInfo{}, nil
					}
				})

				It("spares user managed files", func() {
					err := gc.Remove("some-dir")
					Expect(err).NotTo(HaveOccurred())

					Expect(fileIO.RemoveCall.Receives).NotTo(ContainElement(fakes.RemoveReceive{
						Name: filepath.Join("some-dir", "vars", "user-managed-file"),
					}))
				})

				It("prints a warning about what files are left", func() {
				    err := gc.Remove("some-dir")
					Expect(err).NotTo(HaveOccurred())

					Expect(printer.PrintfCall.Messages).To(ContainElement(filepath.Join("some-dir", "vars", "user-managed-file")))
				})
			})
		})

		Describe("terraform", func() {
			It("removes the bbl template and directory", func() {
				bblTerraformTemplate := filepath.Join("some-dir", "terraform", "bbl-template.tf")

				err := gc.Remove("some-dir")
				Expect(err).NotTo(HaveOccurred())

				Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{Name: bblTerraformTemplate}))
				Expect(fileIO.RemoveCall.Receives).To(ContainElement(fakes.RemoveReceive{
					Name: filepath.Join("some-dir", "terraform"),
				}))
			})

			Context("when the terraform directory contains user managed files", func() {
				BeforeEach(func() {
					fileIO.ReadDirCall.Fake = func(dirname string) ([]os.FileInfo, error) {
						if dirname == "some-dir/terraform" {
							return []os.FileInfo{fakes.FileInfo{FileName: "user-managed.tf"}}, nil
						}
						return []os.FileInfo{}, nil
					}
				})

				It("prints a warning about what files are left", func() {
					err := gc.Remove("some-dir")
					Expect(err).NotTo(HaveOccurred())
					Expect(printer.PrintfCall.Messages).To(ConsistOf(filepath.Join("some-dir", "terraform", "user-managed.tf")))
					Expect(printer.PrintfCall.CallCount).To(Equal(1))
				})
			})
		})

		Context("when the bbl-state.json file does not exist", func() {
			It("does nothing", func() {
				err := gc.Remove("some-dir")
				Expect(err).NotTo(HaveOccurred())

				Expect(len(fileIO.WriteFileCall.Receives)).To(Equal(0))
			})
		})

		Context("failure cases", func() {
			Context("when the bbl-state.json file cannot be removed", func() {
				BeforeEach(func() {
					fileIO.RemoveCall.Returns = []fakes.RemoveReturn{{Error: errors.New("permission denied")}}
				})

				It("returns an error", func() {
					err := gc.Remove("some-dir")
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})
		})
	})
})
