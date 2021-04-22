pragma solidity >=0.7.0 <0.9.0;

import "./token_storage.sol";

// inherit from TokenStorage so we have the constructor, since the token variables need to be stored in the
// proxy's storage
contract Proxy is TokenStorage {
    receive () external payable {
        // directly get the implementation contract address from the storage. This way we don't need to depend
        // on the upgradeable contract
        address _impl = getAddress(keccak256(abi.encode("implementation")));
        require(_impl != address(0), "The implementation address can't be the zero address");

        assembly {
            let ptr := mload(0x40)
            calldatacopy(ptr, returndatasize(), calldatasize())

            let result := delegatecall(gas(), _impl, ptr, calldatasize(), returndatasize(), returndatasize())
            let size := returndatasize()
            returndatacopy(ptr, 0, size)
            switch result
            case 0 { revert(ptr, size) }
            default { return(ptr, size) }
        }
    }

    constructor() public {
        //set initial contract address, needs to be hardcoded
        // TODO: Set correct address
        address impl_addr = address(0xDAD7A460EA562e28fB90cF524B62ea4cBc1685af);
        require(impl_addr != address(0), "implementation address can not be the zero address");
        setAddress(keccak256(abi.encode("implementation")), impl_addr);
        setString(keccak256(abi.encode("version")),"0");

        // set initial owner
        setBool(keccak256(abi.encode("owner", msg.sender)), true);
    }
}