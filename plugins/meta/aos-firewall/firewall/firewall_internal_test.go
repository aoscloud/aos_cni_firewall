// SPDX-License-Identifier: Apache-2.0
//
// Copyright (C) 2021 Renesas Electronics Corporation.
// Copyright (C) 2021 EPAM Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package firewall

import (
	"fmt"
	"net"
	"os"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/coreos/go-iptables/iptables"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

/*******************************************************************************
 * Const
 ******************************************************************************/

const (
	runtimeConfigPath = "/tmp/firewall_test/aos-firewall-test.conf"
)

/*******************************************************************************
 * Tests
 ******************************************************************************/

func listFilterRules(chain string) (rules []string, err error) {
	ipt, err := iptables.New()
	if err != nil {
		return []string{}, err
	}
	rules, err = ipt.List("filter", chain)
	if err != nil {
		return []string{}, err
	}

	return rules[1:], nil
}

func clearFilterRules(chain string) (err error) {
	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	return ipt.ClearChain("filter", chain)
}

var _ = Describe("Firewall", func() {
	var err error
	var fw *Firewall

	interfaceInfoByIP = mockInterfaceInfoByIP

	BeforeEach(func() {
		fw, err = New(runtimeConfigPath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(runtimeConfigPath)
		Expect(err).NotTo(HaveOccurred())

		err = clearFilterRules(forwardChainName)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Test Iptables Rules", func() {
		iconf1 := current.IPConfig{}
		iconf2 := current.IPConfig{}

		iconf1.Address.IP = net.IPv4(10, 0, 0, 2)
		iconf1.Gateway = net.IPv4(10, 0, 0, 1)
		iconf1.Address.Mask = net.IPv4Mask(0xff, 0xff, 0, 0)

		iconf2.Address.IP = net.IPv4(20, 0, 0, 2)
		iconf2.Gateway = net.IPv4(20, 0, 0, 1)
		iconf2.Address.Mask = net.IPv4Mask(0xff, 0xff, 0, 0)

		chain1 := NewAccessChain("AOS_TEST_SERVICE1", "0000-0000-0000-0000", iconf1.Address, iconf1.Gateway, true)
		chain2 := NewAccessChain("AOS_TEST_SERVICE2", "1111-1111-1111-1111", iconf2.Address, iconf2.Gateway, true)

		err = chain1.AddInRule("1001:1002,1005", "tcp")
		Expect(err).NotTo(HaveOccurred())
		err = chain1.AddInRule("1000:1010", "udp")
		Expect(err).NotTo(HaveOccurred())

		err = chain2.AddInRule("2000:2002,2004", "tcp")
		Expect(err).NotTo(HaveOccurred())
		err = chain2.AddInRule("6000", "udp")
		Expect(err).NotTo(HaveOccurred())

		chain1.OutRules = append(chain1.OutRules, AccessRule{
			DstIP:   "20.0.0.2",
			DstPort: "2001",
			SrcIP:   "10.0.0.2",
			Proto:   "tcp",
		})

		chain1.OutRules = append(chain1.OutRules, AccessRule{
			DstIP:   "20.0.0.2",
			DstPort: "2002",
			SrcIP:   "10.0.0.2",
			Proto:   "tcp",
		})

		chain1.OutRules = append(chain1.OutRules, AccessRule{
			DstIP:   "20.0.0.2",
			DstPort: "6000",
			SrcIP:   "10.0.0.2",
			Proto:   "udp",
		})

		chain2.OutRules = append(chain2.OutRules, AccessRule{
			DstIP:   "10.0.0.2",
			DstPort: "1002",
			SrcIP:   "20.0.0.2",
			Proto:   "tcp",
		})

		chain2.OutRules = append(chain2.OutRules, AccessRule{
			DstIP:   "10.0.0.2",
			DstPort: "1003",
			SrcIP:   "20.0.0.2",
			Proto:   "udp",
		})

		err = fw.Add(chain1)
		Expect(err).NotTo(HaveOccurred())

		err = fw.Check(chain1)
		Expect(err).NotTo(HaveOccurred())

		err = fw.Add(chain2)
		Expect(err).NotTo(HaveOccurred())

		err = fw.Check(chain2)
		Expect(err).NotTo(HaveOccurred())

		rulesContainerChain1, err := listFilterRules(chain1.Name)
		Expect(err).NotTo(HaveOccurred())

		rulesContainerChain2, err := listFilterRules(chain2.Name)
		Expect(err).NotTo(HaveOccurred())

		rulesForward, err := listFilterRules(forwardChainName)
		Expect(err).NotTo(HaveOccurred())

		Expect(rulesContainerChain1).To(Equal([]string{
			fmt.Sprintf("-A %s -s 10.0.0.0/16 -p tcp -m tcp -j ACCEPT", chain1.Name),
			fmt.Sprintf("-A %s -s 10.0.0.0/16 -p udp -m udp -j ACCEPT", chain1.Name),
			fmt.Sprintf("-A %s -s 0.0.0.0/16 -p tcp -m tcp -m multiport --dports 1001:1002,1005 -j RETURN", chain1.Name),
			fmt.Sprintf("-A %s -s 0.0.0.0/16 -p udp -m udp -m multiport --dports 1000:1010 -j RETURN", chain1.Name),
			fmt.Sprintf("-A %s -p tcp -m tcp -j DROP", chain1.Name),
			fmt.Sprintf("-A %s -p udp -m udp -j DROP", chain1.Name),
		}))

		Expect(rulesContainerChain2).To(Equal([]string{
			fmt.Sprintf("-A %s -s 20.0.0.0/16 -p tcp -m tcp -j ACCEPT", chain2.Name),
			fmt.Sprintf("-A %s -s 20.0.0.0/16 -p udp -m udp -j ACCEPT", chain2.Name),
			fmt.Sprintf("-A %s -s 0.0.0.0/16 -p tcp -m tcp -m multiport --dports 2000:2002,2004 -j RETURN", chain2.Name),
			fmt.Sprintf("-A %s -s 0.0.0.0/16 -p udp -m udp --dport 6000 -j RETURN", chain2.Name),
			fmt.Sprintf("-A %s -p tcp -m tcp -j DROP", chain2.Name),
			fmt.Sprintf("-A %s -p udp -m udp -j DROP", chain2.Name),
		}))

		Expect(rulesForward).To(Equal([]string{
			// chan1
			fmt.Sprintf("-A %s -s 10.0.0.2/32 -o wan -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -d 10.0.0.2/32 -i wan -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 10.0.0.2/32 -d 20.0.0.2/32 -p tcp -m tcp --dport 2001 -j ACCEPT", forwardChainName),
			fmt.Sprintf(
				"-A %s -s 20.0.0.2/32 -d 10.0.0.2/32 -p tcp -m tcp --sport 2001"+
					" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 10.0.0.2/32 -d 20.0.0.2/32 -p tcp -m tcp --dport 2002 -j ACCEPT", forwardChainName),
			fmt.Sprintf(
				"-A %s -s 20.0.0.2/32 -d 10.0.0.2/32 -p tcp -m tcp --sport 2002"+
					" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 10.0.0.2/32 -d 20.0.0.2/32 -p udp -m udp --dport 6000 -j ACCEPT", forwardChainName),
			fmt.Sprintf(
				"-A %s -s 20.0.0.2/32 -d 10.0.0.2/32 -p udp -m udp --sport 6000"+
					" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 10.0.0.2/32 -d 10.0.0.0/16 -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 10.0.0.0/16 -d 10.0.0.2/32 -j ACCEPT", forwardChainName),
			// chan2
			fmt.Sprintf("-A %s -s 20.0.0.2/32 -o wan -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -d 20.0.0.2/32 -i wan -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 20.0.0.2/32 -d 10.0.0.2/32 -p tcp -m tcp --dport 1002 -j ACCEPT", forwardChainName),
			fmt.Sprintf(
				"-A %s -s 10.0.0.2/32 -d 20.0.0.2/32 -p tcp -m tcp --sport 1002"+
					" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 20.0.0.2/32 -d 10.0.0.2/32 -p udp -m udp --dport 1003 -j ACCEPT", forwardChainName),
			fmt.Sprintf(
				"-A %s -s 10.0.0.2/32 -d 20.0.0.2/32 -p udp -m udp --sport 1003"+
					" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 20.0.0.2/32 -d 20.0.0.0/16 -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 20.0.0.0/16 -d 20.0.0.2/32 -j ACCEPT", forwardChainName),
		}))

		err = fw.Del(chain1.ContainerID)
		Expect(err).NotTo(HaveOccurred())

		err = fw.Check(chain1)
		Expect(err).To(HaveOccurred())

		_, err = listFilterRules(chain1.Name)
		Expect(err).To(HaveOccurred())

		rulesForward, err = listFilterRules(forwardChainName)
		Expect(err).NotTo(HaveOccurred())

		Expect(rulesForward).To(Equal([]string{
			// chan2
			fmt.Sprintf("-A %s -s 20.0.0.2/32 -o wan -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -d 20.0.0.2/32 -i wan -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 20.0.0.2/32 -d 10.0.0.2/32 -p tcp -m tcp --dport 1002 -j ACCEPT", forwardChainName),
			fmt.Sprintf(
				"-A %s -s 10.0.0.2/32 -d 20.0.0.2/32 -p tcp -m tcp --sport 1002"+
					" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 20.0.0.2/32 -d 10.0.0.2/32 -p udp -m udp --dport 1003 -j ACCEPT", forwardChainName),
			fmt.Sprintf(
				"-A %s -s 10.0.0.2/32 -d 20.0.0.2/32 -p udp -m udp --sport 1003"+
					" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 20.0.0.2/32 -d 20.0.0.0/16 -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 20.0.0.0/16 -d 20.0.0.2/32 -j ACCEPT", forwardChainName),
		}))

		err = fw.Del(chain2.ContainerID)
		Expect(err).NotTo(HaveOccurred())

		err = fw.Check(chain1)
		Expect(err).To(HaveOccurred())

		_, err = listFilterRules(chain2.Name)
		Expect(err).To(HaveOccurred())

		rulesForward, err = listFilterRules(forwardChainName)
		Expect(err).NotTo(HaveOccurred())

		Expect(rulesForward).To(Equal([]string{}))
	})

	It("No exposed ports were provided", func() {
		var iconf3 current.IPConfig

		iconf3.Address.IP = net.IPv4(30, 0, 0, 2)
		iconf3.Gateway = net.IPv4(30, 0, 0, 1)
		iconf3.Address.Mask = net.IPv4Mask(0xff, 0xff, 0, 0)

		// No in rules were provided
		chain3 := NewAccessChain("AOS_TEST_SERVICE3", "3333-3333-3333-3333", iconf3.Address, iconf3.Gateway, true)

		err = fw.Add(chain3)
		Expect(err).NotTo(HaveOccurred())

		err = fw.Check(chain3)
		Expect(err).NotTo(HaveOccurred())

		rulesContainerChain3, err := listFilterRules(chain3.Name)
		Expect(err).NotTo(HaveOccurred())

		rulesForward, err := listFilterRules(forwardChainName)
		Expect(err).NotTo(HaveOccurred())

		Expect(rulesContainerChain3).To(Equal([]string{
			fmt.Sprintf("-A %s -s 30.0.0.0/16 -p tcp -m tcp -j ACCEPT", chain3.Name),
			fmt.Sprintf("-A %s -s 30.0.0.0/16 -p udp -m udp -j ACCEPT", chain3.Name),
			fmt.Sprintf("-A %s -p tcp -m tcp -j DROP", chain3.Name),
			fmt.Sprintf("-A %s -p udp -m udp -j DROP", chain3.Name),
		}))

		Expect(rulesForward).To(Equal([]string{
			fmt.Sprintf("-A %s -s 30.0.0.2/32 -o wan -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -d 30.0.0.2/32 -i wan -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 30.0.0.2/32 -d 30.0.0.0/16 -j ACCEPT", forwardChainName),
			fmt.Sprintf("-A %s -s 30.0.0.0/16 -d 30.0.0.2/32 -j ACCEPT", forwardChainName),
		}))

		err = fw.Del(chain3.ContainerID)
		Expect(err).NotTo(HaveOccurred())

		_, err = listFilterRules(chain3.Name)
		Expect(err).To(HaveOccurred())

		rulesForward, err = listFilterRules(forwardChainName)
		Expect(err).NotTo(HaveOccurred())

		Expect(rulesForward).To(Equal([]string{}))
	})
})

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func mockInterfaceInfoByIP(ip net.IP) (name string, mask string, err error) {
	return "wan", "16", nil
}
