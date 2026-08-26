# Architecture

The xk6-disruptor consists of two main components: a k6 extension and the xk6-disruptor-agent. The xk6-disruptor-agent is a command line tool that can inject disruptions in the target system where it runs. The xk6-disruptor extension provides an API for injecting faults into a target system using the xk6-disruptor as a backend tool. The xk6-disruptor extension will install the agent in the target and send commands in order to inject the desired faults.

The xk6-disruptor-agent is provided as an Docker image that can be pulled from the [xk6-disruptor repository](https://github.com/grafana/xk6-disruptor/pkgs/container/xk6-disruptor-agent) as or [build locally](./01-contributing.md#building-the-xk6-disruptor-agent-image).


The agent offers a series of commands that inject different types of disruptions. It can run standalone, as a CLI application to facilitate debugging.

