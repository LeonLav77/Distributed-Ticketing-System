import TICKET_ABI from './MintNFTABI.js';
import MARKETPLACE_ABI from './MarketplaceABI.js';
import { 
    TICKET_CONTRACT_ADDRESS, 
    MARKETPLACE_CONTRACT_ADDRESS,
    connectMetaMask,
    getWeb3,
    fetchMetadata,
    getConnectedWallet,
    onAccountsChanged
} from './shared.js';

const web3 = getWeb3();
const marketplaceContract = new web3.eth.Contract(MARKETPLACE_ABI, MARKETPLACE_CONTRACT_ADDRESS);
const ticketContract = new web3.eth.Contract(TICKET_ABI, TICKET_CONTRACT_ADDRESS);

function marketplaceApp() {
    return {
        loading: true,
        error: null,
        walletConnected: false,
        walletAddress: '',
        activeTab: 'browse',
        listings: [],
        myTickets: [],
        showModal: false,
        modalTicket: null,
        modalPrice: '',

        get myListings() {
            return this.listings.filter(listing => 
                listing.seller.toLowerCase() === this.walletAddress.toLowerCase()
            );
        },

        async init() {
            const wallet = await getConnectedWallet();
            if (wallet) {
                this.walletAddress = wallet;
                this.walletConnected = true;
            }

            onAccountsChanged((accounts) => {
                this.walletAddress = accounts[0] || '';
                this.walletConnected = !!accounts[0];
                if (this.walletConnected) { 
                    this.loadData();
                }else{ 
                    this.myTickets = [];
                }
            });

            await this.loadData();
        },

        async connectWallet() {
            try {
                this.walletAddress = await connectMetaMask();
                this.walletConnected = true;
                await this.loadData();
            } catch (error) {
                alert('Failed to connect wallet: ' + error.message);
            }
        },

        async loadData() {
            this.loading = true;
            this.error = null;

            await this.loadListings();
            
            if (this.walletConnected) {
                await this.loadMyTickets();
            }
        },

        async loadListings() {
            const events = await marketplaceContract.getPastEvents('TicketListed', {
                fromBlock: 0,
                toBlock: 'latest'
            });

            let eventsTokenIds = events.map(e => e.returnValues.tokenId);
            const tokenIds = [...new Set(eventsTokenIds)];

            const listings = [];
            for (const tokenId of tokenIds) {
                const listing = await marketplaceContract.methods.listings(tokenId).call();
                
                if (listing.active) {
                    const tokenURI = await ticketContract.methods.tokenURI(tokenId).call();
                    const metadata = await fetchMetadata(tokenURI);

                    listings.push({
                        tokenId,
                        seller: listing.seller,
                        price: listing.price,
                        priceETH: web3.utils.fromWei(listing.price, 'ether'),
                        metadata
                    });
                }
            }

            this.listings = listings.filter(Boolean);
        },

        async loadMyTickets() {
            const events = await ticketContract.getPastEvents('Transfer', {
                filter: { to: this.walletAddress },
                fromBlock: 0,
                toBlock: 'latest'
            });

            let eventsTokenIds = events.map(e => e.returnValues.tokenId);
            const tokenIds = [...new Set(eventsTokenIds)];

            const tickets = [];

            for (const tokenId of tokenIds) {
                const owner = await ticketContract.methods.ownerOf(tokenId).call();
                if (owner.toLowerCase() !== this.walletAddress.toLowerCase()) continue;

                const tokenURI = await ticketContract.methods.tokenURI(tokenId).call();
                const metadata = await fetchMetadata(tokenURI);
                
                let listing = { active: false };
                listing = await marketplaceContract.methods.listings(tokenId).call();

                tickets.push({
                    tokenId,
                    metadata,
                    isListed: listing.active && listing.seller.toLowerCase() === this.walletAddress.toLowerCase(),
                    listingPrice: listing.active ? web3.utils.fromWei(listing.price, 'ether') : '0'
                });
            }

            this.myTickets = tickets.filter(Boolean);
        },

        showListModal(ticket) {
            this.modalTicket = ticket;
            this.modalPrice = ticket.listingPrice || '';
            this.showModal = true;
        },

        showUpdatePriceModal(listing) {
            this.modalTicket = listing;
            this.modalPrice = listing.priceETH;
            this.showModal = true;
        },

        async confirmListTicket() {
            if (!this.modalPrice || parseFloat(this.modalPrice) <= 0) {
                alert('Please enter a valid price');
                return;
            }

            try {
                const priceWei = web3.utils.toWei(this.modalPrice, 'ether');
                const isApproved = await ticketContract.methods.isApprovedForAll(
                    this.walletAddress, 
                    MARKETPLACE_CONTRACT_ADDRESS
                ).call();

                if (!isApproved) {
                    alert('First approval required for marketplace access (one-time)');
                    await ticketContract.methods.setApprovalForAll(MARKETPLACE_CONTRACT_ADDRESS, true)
                        .send({ from: this.walletAddress });
                }

                await marketplaceContract.methods.listTicket(this.modalTicket.tokenId, priceWei)
                    .send({ from: this.walletAddress });

                alert('Ticket listed successfully!');
                this.showModal = false;
                await this.loadData();
            } catch (error) {
                alert('Failed to list ticket: ' + error.message);
            }
        },

        async cancelListing(tokenId) {
            if (!confirm('Cancel this listing?')) return;

            await marketplaceContract.methods.cancelListing(tokenId).send({ from: this.walletAddress });
            await this.loadData();
        },

        async buyTicket(listing) {
            if (!this.walletConnected) {
                alert('Connect wallet first');
                return;
            }

            await marketplaceContract.methods.buyTicket(listing.tokenId).send({ from: this.walletAddress, value: listing.price });
            await this.loadData();
        }
    };
}

window.marketplaceApp = marketplaceApp;