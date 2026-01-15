// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";

contract TicketToNFT is ERC721URIStorage {
    uint256 private tokenIdCounter;
    mapping(string => bool) public ticketMinted;
    constructor() ERC721("Event Ticket", "TICKET") {}

    function mintBatch(
        address to,
        string[] memory uris,
        string memory orderReferenceId
    ) public returns (uint256[] memory) {
        checkIfOrderMinted(orderReferenceId);

        uint256[] memory ids = new uint256[](uris.length);

        for(uint16 i = 0; i < uris.length; i++){
            uint256 id = tokenIdCounter++;
            _safeMint(to, id);
            _setTokenURI(id, uris[i]);
            ids[i] = id;
        }

        ticketMinted[orderReferenceId] = true;
        return ids;
    }

    function checkIfOrderMinted(string memory orderReferenceId) internal view {
        require(!ticketMinted[orderReferenceId], "Order already minted as NFT");
    }
}