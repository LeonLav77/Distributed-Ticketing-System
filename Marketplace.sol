// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.33;

import "@openzeppelin/contracts/token/ERC721/IERC721.sol";

interface ITicketToNFT {
    function getPerformer(uint256 tokenId) external view returns (address);
}

contract TicketMarketplace {
    struct Listing {
        address seller;
        uint256 price;
        bool active;
    }

    ITicketToNFT public ticketContract;
    uint256 public platformFeePercent;
    uint256 public performerFeePercent;
    address public platformWallet;

    mapping(uint256 => Listing) public listings;

    event TicketListed(uint256 indexed tokenId, address indexed seller, uint256 price);
    event ListingCancelled(uint256 indexed tokenId);
    event ListingPriceUpdated(uint256 indexed tokenId, uint256 newPrice);
    event TicketSold(uint256 indexed tokenId, address indexed buyer, address indexed seller, uint256 price);

    constructor(address _ticketContract, uint256 _platformFeePercent, uint256 _performerFeePercent, address _platformWallet) {
        ticketContract = ITicketToNFT(_ticketContract);
        platformFeePercent = _platformFeePercent;
        performerFeePercent = _performerFeePercent;
        platformWallet = _platformWallet;
    }

    function listTicket(uint256 tokenId, uint256 price) external {
        checkIfSenderOwnsTicket(tokenId);
        checkIfPriceIsValid(price);

        listings[tokenId] = Listing({
            seller: msg.sender,
            price: price,
            active: true
        });

        emit TicketListed(tokenId, msg.sender, price);
    }

    function cancelListing(uint256 tokenId) external {
        checkIfSenderIsSeller(tokenId);
        listings[tokenId].active = false;
        emit ListingCancelled(tokenId);
    }

    function updateListingPrice(uint256 tokenId, uint256 newPrice) external {
        checkIfSenderIsSeller(tokenId);
        checkIfListingIsActive(tokenId);
        checkIfPriceIsValid(newPrice);

        listings[tokenId].price = newPrice;
        emit ListingPriceUpdated(tokenId, newPrice);
    }

    function buyTicket(uint256 tokenId) external payable {
        Listing memory listing = listings[tokenId];
        checkIfListingIsActive(tokenId);
        checkIfPaymentIsCorrect(tokenId);

        address performer = ticketContract.getPerformer(tokenId);

        uint256 platformFee = (listing.price * platformFeePercent) / 10000;
        uint256 performerFee = (listing.price * performerFeePercent) / 10000;
        uint256 sellerAmount = listing.price - platformFee - performerFee;

        listings[tokenId].active = false;
        IERC721(address(ticketContract)).transferFrom(listing.seller, msg.sender, tokenId);

        payable(listing.seller).transfer(sellerAmount);
        payable(platformWallet).transfer(platformFee);
        payable(performer).transfer(performerFee);

        emit TicketSold(tokenId, msg.sender, listing.seller, listing.price);
    }

    function checkIfSenderOwnsTicket(uint256 tokenId) internal view {
        require(IERC721(address(ticketContract)).ownerOf(tokenId) == msg.sender, "Not the owner");
    }

    function checkIfPriceIsValid(uint256 price) internal pure {
        require(price > 0, "Price must be greater than 0");
    }

    function checkIfSenderIsSeller(uint256 tokenId) internal view {
        require(listings[tokenId].seller == msg.sender, "Not the seller");
    }

    function checkIfListingIsActive(uint256 tokenId) internal view {
        require(listings[tokenId].active, "Listing not active");
    }

    function checkIfPaymentIsCorrect(uint256 tokenId) internal view {
        require(msg.value == listings[tokenId].price, "Incorrect payment");
    }
}