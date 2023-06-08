// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.19;

import "@openzeppelin/contracts/access/Ownable.sol";

contract accountactivation is Ownable {
    address payable private _beneficiary;
    mapping(string  => uint256) private _costs;

    event BeneficiaryChanged(address indexed previousBeneficiary, address indexed newBeneficiary);
    event NetworkCostChanged(string network, uint256 cost);


    constructor() {
        // set initial owner
        Ownable(msg.sender);
        // set initial beneficiary
        _beneficiary = payable(msg.sender);
    }

    /**
     * Getter for the beneficiary address.
     */
    function beneficiary() public view returns (address payable) {
        return _beneficiary;
    }

    /**
     * Changes the beneficiary of the contract to a new account (`newBeneficiary`).
     * Can only be called by the owner.
     */
    function changeBeneficiary(address payable newBeneficiary) public  onlyOwner {
        require(newBeneficiary != address(0), "new beneficiary is the zero address");
        address previousBeneficiary = _beneficiary;
        _beneficiary = newBeneficiary;
        emit BeneficiaryChanged(previousBeneficiary, newBeneficiary);
    }

    /**
     * returns the cost in Wei for activating an account on a `network`.
     */
    function networkCost(string memory network) public view returns (uint256) {
        return _costs[network];
    }
    
    /**
     * sets the cost in Wei for activating an account on a `network` 
     */
    function setNetworkCost(string calldata network,uint256 cost) external onlyOwner {
        _costs[network] = cost;
        emit NetworkCostChanged(network, cost);
    }

    //Activate an account on a different blockchain (defined by the network parameter) 
    event ActivateAccount(string network, string account);
    function activateAccount(string calldata network, string calldata account ) external payable {
        uint256 cost = _costs[network];
        require(msg.value >= cost,"Ether sent is maller than the cost for the network");
        if (cost > 0){
            (bool sent, ) = _beneficiary.call{value: msg.value}("");
            require(sent, "Failed to send Ether");
        }
        emit ActivateAccount(network,account);
    }



}