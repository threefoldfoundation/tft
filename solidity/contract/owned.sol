pragma solidity >=0.7.0 <0.9.0;

import "./storage.sol";

contract Owned is Storage {
    
    // -----------------------------------------------------
    // Usual storage
    // -----------------------------------------------------

    // mapping(address => bool) public owner;

    // -----------------------------------------------------
    // Events
    // -----------------------------------------------------

    event AddedOwner(address newOwner);
    event RemovedOwner(address removedOwner);

    address[] public owners;
    mapping (address => bool) public isOwner;

    // -----------------------------------------------------
    // storage utilities
    // -----------------------------------------------------

    function _isOwner(address _caller) internal view returns (bool) {
        return getBool(keccak256(abi.encode("owner",_caller)));
    }

    function _addOwner(address _newOwner) internal {
        setBool(keccak256(abi.encode("owner", _newOwner)), true);
        isOwner[_newOwner] = true;
        owners.push(_newOwner);
    }

    function _deleteOwner(address _owner) internal {
        isOwner[_owner] = false;
        for (uint i=0; i<owners.length - 1; i++)
            if (owners[i] == _owner) {
                owners[i] = owners[owners.length - 1];
                break;
            }
        owners.pop();

        deleteBool(keccak256(abi.encode("owner", _owner)));
    }

    // -----------------------------------------------------
    // Main contract
    // -----------------------------------------------------

    constructor() {
        _addOwner(msg.sender);
    }

    modifier onlyOwner() {
        require(_isOwner(msg.sender));
        _;
    }

    function addOwner(address _newOwner) onlyOwner public {
        require(_newOwner != address(0));
        _addOwner(_newOwner);
        emit AddedOwner(_newOwner);
    }

    function removeOwner(address _toRemove) onlyOwner public {
        require(_toRemove != address(0));
        require(_toRemove != msg.sender);
        _deleteOwner(_toRemove);
        emit RemovedOwner(_toRemove);
    }

}