# ephemerad
Ephemerad is the service that will listen on the bus and run the
ephemeral scripts. This service is used in conjunction with the
existing VCI tools to generate the illusion that all the components
are persistent and running on the bus. It does this by acting as a
proxy for VCI requests.

## Component definitions
Ephemeral components are special in the eyes of the VCI helpers so
their component definitions are slightly different than normal components.
For ephemeral components one does not specify ExecName but instead says
'Ephemeral=true'. An example is included below.

```
[Vyatta Component]
Name=net.vyatta.eng.vci.example.ephemeral.toaster
Description=Ephemeral version of the VCI Toaster
Ephemeral=true
ConfigFile=/etc/toaster.conf

[Model net.vyatta.eng.vci.example.ephemeral.toaster.v1]
Modules=toaster
ModelSets=vyatta-v1
```

## Instance definitions
Ephemerad needs to know what script to call when performing an action
for the component. For this we use an instance definition such as the
one below. Instance definitions are expected to be installed in the
'/lib/vci/ephemera/instances' directory.

```
[Component]
Name=net.vyatta.eng.vci.example.ephemeral.toaster
Start=/lib/vci-toaster-ephemeral --action=start
Stop=/lib/vci-toaster-ephemeral --action=stop

[Model net.vyatta.eng.vci.example.ephemeral.toaster.v1]
Config/Check=/lib/vci-toaster-ephemeral/vci-toaster --action=validate
Config/Set=/lib/vci-toaster-ephemeral/vci-toaster --action=commit
Config/Get=/lib/vci-toaster-ephemeral/vci-toaster --action=get-config
State/Get=/lib/vci-toaster-ephemeral/vci-toaster --action=get-state
RPC/toaster/make-toast=/lib/vci-toaster-ephemeral/vci-toaster --action=make-toast
RPC/toaster/cancel-toast=/lib/vci-toaster-ephemeral/vci-toaster --action=cancel-toast
RPC/toaster/restock-toaster=/lib/vci-toaster-ephemeral/vci-toaster --action=restock-toaster
```

This instance definition tells ephemerad how to call the scripts when
bus actions are called. There is one instance definition per managed
component. The instance definitions are installed in
'/lib/vci/ephemera/instances'.

## The script environement
Scripts are called using the UNIX environment and standard interfaces for interaction. The environment will be setup as follows.

| UNIX interface  | Function |
| --------------  | -------- |
| stdin           | The rfc7951 encoded data for the action (if any). |
| stdout          | The rfc7951 encoded output for the action (if any). |
| stderr          | Error output from the action, may be rfc7951 encoded YANG messages or strings. |
| VCI_COMPONENT_NAME | Name of the component. |
| VCI_MODEL_NAME  | Name of the model. |
| VCI_RPC_METADATA | The json encoded metadata associated with an RPC call. |
| EPHEMERA_MESSAGE| The statement from the instance file that is being invoked. 'Config/Get', 'RPC/module/name', etc. |
| exit code       | Determins whether the script had an error (0 success; non-0 failure) |


## Conclusion
Ephemeral components allow for hopefully an easier transition for
certain features to VCI. The ephemeral components will use
considerably less resident memory than a full fledged component and
are appropriate for small features.
